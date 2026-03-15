package hippod

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Noksa/operator-home/pkg/operatorkclient"
	"github.com/fatih/color"
	"github.com/noksa/helm-in-pod/internal/cmdoptions"
	"github.com/noksa/helm-in-pod/internal/helmtar"
	"github.com/noksa/helm-in-pod/internal/hipconsts"
	"github.com/noksa/helm-in-pod/internal/hipretry"
	"github.com/noksa/helm-in-pod/internal/logz"
	log "github.com/sirupsen/logrus"
	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const Namespace = "helm-in-pod"

type Manager struct {
	ctx         context.Context
	myHostname  string
	interrupted bool
}

func NewManager(ctx context.Context, hostname string) *Manager {
	return &Manager{
		ctx:        ctx,
		myHostname: hostname,
	}
}

func (m *Manager) client() *operatorkclient.Client {
	return operatorkclient.DefaultClient()
}

func (m *Manager) DeleteHelmPods(execOptions cmdoptions.ExecOptions, purgeOptions cmdoptions.PurgeOptions) error {
	opts := metav1.ListOptions{}
	if !purgeOptions.All {
		selector := fmt.Sprintf("host=%v", m.myHostname)
		for k, v := range execOptions.Labels {
			selector = fmt.Sprintf("%v,%v=%v", selector, k, v)
		}
		selector = strings.TrimSuffix(selector, ",")
		selector = strings.TrimPrefix(selector, ",")
		opts.LabelSelector = selector
	}
	pods, err := m.client().ClientSet().CoreV1().Pods(Namespace).List(context.Background(), opts)
	if err != nil {
		return err
	}
	for _, pod := range pods.Items {
		log.Debugf("%v Deleting '%v' pod", logz.LogHost(), pod.Name)

		// Extract operation ID from pod labels and delete associated PDB
		if operationID, ok := pod.Labels[hipconsts.LabelOperationID]; ok {
			if err := m.DeletePodDisruptionBudgets(context.Background(), operationID); err != nil {
				log.Warnf("Failed to delete PodDisruptionBudget for operation %s: %v", operationID, err)
			}
		}

		err = m.client().ClientSet().CoreV1().Pods(Namespace).Delete(context.Background(), pod.Name, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
		log.Debugf("%v '%v' pod has been deleted", logz.LogHost(), pod.Name)
	}
	return nil
}

func (m *Manager) CreateHelmPod(opts cmdoptions.ExecOptions) (*corev1.Pod, error) {
	err := m.DeleteHelmPods(opts, cmdoptions.PurgeOptions{All: false})
	if err != nil {
		return nil, err
	}
	log.Infof("%v Creating '%v' pod", logz.LogHost(), Namespace)

	podSpec, err := buildPodSpec(opts, false)
	if err != nil {
		return nil, err
	}

	// Generate unique operation ID for this pod
	operationID := GenerateOperationID()

	labels := map[string]string{
		"host":                     m.myHostname,
		hipconsts.LabelOperationID: operationID,
	}
	maps.Copy(labels, opts.Labels)
	annotations := map[string]string{}
	maps.Copy(annotations, opts.Annotations)

	pod, err := m.client().ClientSet().CoreV1().Pods(Namespace).Create(m.ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%v-", Namespace),
			Labels:       labels,
			Annotations:  annotations,
		},
		Spec: podSpec,
	}, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	// Create PodDisruptionBudget for this pod if enabled
	if opts.CreatePDB {
		if err := m.CreatePodDisruptionBudget(m.ctx, operationID); err != nil {
			// If PDB creation fails, clean up the pod
			_ = m.client().ClientSet().CoreV1().Pods(Namespace).Delete(m.ctx, pod.Name, metav1.DeleteOptions{})
			return nil, fmt.Errorf("failed to create PodDisruptionBudget: %w", err)
		}
	}

	// Handle interrupt signals
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		if pod != nil && pod.Name != "" {
			log.Warnf("%v Interrupted! Destroying helm pod", logz.LogHost())
			destroyErr := m.DeleteHelmPods(opts, cmdoptions.PurgeOptions{All: false})
			if destroyErr != nil {
				log.Errorf("Couldn't destroy helm pods: %v", destroyErr.Error())
			}
			// Clean up PDB if it was created
			if opts.CreatePDB {
				_ = m.DeletePodDisruptionBudgets(m.ctx, operationID)
			}
			m.interrupted = true
		}
		<-c
		os.Exit(1)
	}()

	log.Debugf("%v %v pod has been created", logz.LogHost(), color.CyanString(pod.Name))
	return pod, m.waitUntilPodIsRunning(pod)
}

func (m *Manager) waitUntilPodIsRunning(pod *corev1.Pod) error {
	log.Infof("%v Waiting until %v pod is ready", logz.LogHost(), color.CyanString(pod.Name))

	timeout := time.Minute * 5
	start := time.Now()

	for time.Since(start) <= timeout {
		stdout, stderr, err := m.client().RunCommandInPod("[ -f /tmp/ready ] && echo ready", Namespace, pod.Name, pod.Namespace, nil)
		if err == nil && strings.Contains(stdout, "ready") {
			log.Debugf("%v %v pod is ready", logz.LogHost(), color.CyanString(pod.Name))
			return nil
		}

		if m.interrupted {
			return fmt.Errorf("interrupted while was waiting for pod readiness")
		}

		if err != nil {
			log.Debugf("%v Not ready yet: %v %v", logz.LogPod(), stderr, err.Error())
		}

		time.Sleep(time.Second)
	}

	return fmt.Errorf("timeout waiting pod readiness")
}

func (m *Manager) waitUntilPodIsDeleted(podName string) error {
	log.Debugf("%v Waiting for pod %v to be deleted", logz.LogHost(), color.CyanString(podName))

	timeout := time.Minute * 2
	start := time.Now()

	for time.Since(start) <= timeout {
		_, err := m.client().ClientSet().CoreV1().Pods(Namespace).Get(m.ctx, podName, metav1.GetOptions{})
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				log.Infof("%v Pod %v has been deleted", logz.LogHost(), color.CyanString(podName))
				return nil
			}
			return fmt.Errorf("error checking pod status: %w", err)
		}

		if m.interrupted {
			return fmt.Errorf("interrupted while waiting for pod deletion")
		}

		time.Sleep(time.Second)
	}

	return fmt.Errorf("timeout waiting for pod deletion")
}

func (m *Manager) CopyFileToPod(pod *corev1.Pod, srcPath string, destPath string, attempts int) error {
	buffer := &bytes.Buffer{}
	srcPath = filepath.Clean(srcPath)
	destPath = filepath.Clean(destPath)
	err := helmtar.Compress(srcPath, destPath, buffer)
	if err != nil {
		return err
	}

	dir := filepath.Dir(destPath)
	req := m.client().ClientSet().CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(pod.Namespace).
		SubResource("exec").
		Param("container", Namespace)

	req.VersionedParams(&corev1.PodExecOptions{
		Container: Namespace,
		Command: []string{"sh", "-ceu", fmt.Sprintf(`
mkdir -p %v
tar zxf - -C /`, dir)},
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	}, scheme.ParameterCodec)

	return hipretry.Retry(attempts, func() error {
		exec, err := remotecommand.NewSPDYExecutor(m.client().Config(), "POST", req.URL())
		if err != nil {
			return err
		}

		log.Infof("%v %v Copying %v to %v", logz.LogHost(), logz.LogPod(), color.CyanString(srcPath), color.MagentaString(destPath))
		b := &strings.Builder{}
		err = exec.StreamWithContext(m.ctx, remotecommand.StreamOptions{
			Stdin:  bytes.NewReader(buffer.Bytes()),
			Stdout: b,
			Stderr: b,
			Tty:    false,
		})
		if err != nil {
			return multierr.Append(err, fmt.Errorf("%s", b.String()))
		}
		log.Debugf("%v %v %v has been copied to %v", logz.LogHost(), logz.LogPod(), color.CyanString(srcPath), color.MagentaString(destPath))
		return nil
	})
}

// CopyFileFromPod copies a file or directory from the pod to the local host.
// It streams a tar archive from the pod and extracts it locally.
func (m *Manager) CopyFileFromPod(pod *corev1.Pod, podPath string, hostPath string, attempts int) error {
	podPath = filepath.Clean(podPath)
	hostPath = filepath.Clean(hostPath)

	return hipretry.Retry(attempts, func() error {
		req := m.client().ClientSet().CoreV1().RESTClient().Post().
			Resource("pods").
			Name(pod.Name).
			Namespace(pod.Namespace).
			SubResource("exec").
			Param("container", Namespace)

		req.VersionedParams(&corev1.PodExecOptions{
			Container: Namespace,
			Command:   []string{"tar", "czf", "-", "-C", filepath.Dir(podPath), filepath.Base(podPath)},
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
		}, scheme.ParameterCodec)

		exec, err := remotecommand.NewSPDYExecutor(m.client().Config(), "POST", req.URL())
		if err != nil {
			return err
		}

		log.Infof("%v %v Copying %v to %v", logz.LogHost(), logz.LogPod(), color.MagentaString(podPath), color.CyanString(hostPath))

		var stdout bytes.Buffer
		errBuf := &strings.Builder{}
		err = exec.StreamWithContext(m.ctx, remotecommand.StreamOptions{
			Stdout: &stdout,
			Stderr: errBuf,
			Tty:    false,
		})
		if err != nil {
			return multierr.Append(err, fmt.Errorf("%s", errBuf.String()))
		}

		// Extract the tar.gz archive to the host path
		if err := extractTarGz(&stdout, hostPath); err != nil {
			return fmt.Errorf("failed to extract archive: %w", err)
		}

		log.Debugf("%v %v %v has been copied to %v", logz.LogHost(), logz.LogPod(), color.MagentaString(podPath), color.CyanString(hostPath))
		return nil
	})
}

// extractTarGz extracts a gzipped tar archive from r into destDir.
func extractTarGz(r io.Reader, destDir string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(destDir, filepath.Clean(header.Name))

		// Prevent path traversal
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)) {
			return fmt.Errorf("invalid tar entry path: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				_ = f.Close()
				return err
			}
			_ = f.Close()
		}
	}
	return nil
}

func (m *Manager) StreamLogsFromPod(ctx context.Context, pod *corev1.Pod, writer io.Writer, since time.Time) error {
	req := m.client().ClientSet().CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
		Follow:    true,
		SinceTime: &metav1.Time{Time: since},
	})
	stream, err := req.Stream(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = stream.Close() }()
	r := bufio.NewReader(stream)
	for {
		line, err := r.ReadBytes('\n')
		if len(line) != 0 {
			_, _ = writer.Write(line)
		}
		if err != nil {
			if err != io.EOF {
				return err
			}
			return nil
		}
	}
}

func (m *Manager) GetPodPhase(ctx context.Context, pod *corev1.Pod) (corev1.PodPhase, error) {
	myPod, err := m.client().ClientSet().CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
	if err != nil {
		return corev1.PodFailed, client.IgnoreNotFound(err)
	}
	return myPod.Status.Phase, nil
}

func (m *Manager) CreateDaemonPod(opts cmdoptions.DaemonOptions) (*corev1.Pod, error) {
	// Check if daemon pod already exists
	_, err := m.GetDaemonPod(opts.Name)
	if err == nil {
		if !opts.Force {
			return nil, fmt.Errorf("daemon pod '%s' already exists. Use --force to recreate", opts.Name)
		}
		log.Infof("%v Force flag enabled, recreating daemon pod %v", logz.LogHost(), color.CyanString(opts.Name))
		if err := m.DeleteDaemonPod(opts.Name); err != nil {
			return nil, fmt.Errorf("failed to delete existing daemon pod: %w", err)
		}
	}

	log.Infof("%v Creating daemon pod '%v'", logz.LogHost(), opts.Name)

	podSpec, err := buildDaemonPodSpec(opts.ExecOptions)
	if err != nil {
		return nil, err
	}

	// Generate unique operation ID for this daemon pod
	operationID := GenerateOperationID()

	labels := map[string]string{
		"daemon":                   opts.Name,
		hipconsts.LabelOperationID: operationID,
	}
	maps.Copy(labels, opts.Labels)
	annotations := map[string]string{}
	maps.Copy(annotations, opts.Annotations)

	pod, err := m.client().ClientSet().CoreV1().Pods(Namespace).Create(m.ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("daemon-%s", opts.Name),
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: podSpec,
	}, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	// Create PodDisruptionBudget for this daemon pod if enabled
	if opts.CreatePDB {
		if err := m.CreatePodDisruptionBudget(m.ctx, operationID); err != nil {
			// If PDB creation fails, clean up the pod
			_ = m.client().ClientSet().CoreV1().Pods(Namespace).Delete(m.ctx, pod.Name, metav1.DeleteOptions{})
			return nil, fmt.Errorf("failed to create PodDisruptionBudget: %w", err)
		}
	}

	log.Debugf("%v Daemon pod %v has been created", logz.LogHost(), pod.Name)
	return pod, m.waitUntilPodIsRunning(pod)
}

func (m *Manager) GetDaemonPod(name string) (*corev1.Pod, error) {
	podName := fmt.Sprintf("daemon-%s", name)
	pod, err := m.client().ClientSet().CoreV1().Pods(Namespace).Get(m.ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("daemon pod '%s' not found: %w", name, err)
	}
	return pod, nil
}

func (m *Manager) DeleteDaemonPod(name string) error {
	podName := fmt.Sprintf("daemon-%s", name)
	log.Infof("%v Deleting daemon pod %v", logz.LogHost(), color.CyanString(podName))

	// Get the pod to extract operation ID before deletion
	pod, err := m.client().ClientSet().CoreV1().Pods(Namespace).Get(m.ctx, podName, metav1.GetOptions{})
	if err == nil {
		// Extract operation ID from pod labels and delete associated PDB
		if operationID, ok := pod.Labels[hipconsts.LabelOperationID]; ok {
			if err := m.DeletePodDisruptionBudgets(m.ctx, operationID); err != nil {
				log.Warnf("Failed to delete PodDisruptionBudget for operation %s: %v", operationID, err)
			}
		}
	}

	err = m.client().ClientSet().CoreV1().Pods(Namespace).Delete(m.ctx, podName, metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	return m.waitUntilPodIsDeleted(podName)
}

func (m *Manager) AnnotatePod(pod *corev1.Pod, annotations map[string]string) error {
	return hipretry.Retry(3, func() error {
		// Get latest pod state before each attempt
		latestPod, err := m.client().ClientSet().CoreV1().Pods(pod.Namespace).Get(m.ctx, pod.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if latestPod.Annotations == nil {
			latestPod.Annotations = make(map[string]string)
		}
		maps.Copy(latestPod.Annotations, annotations)

		updatedPod, err := m.client().ClientSet().CoreV1().Pods(latestPod.Namespace).Update(m.ctx, latestPod, metav1.UpdateOptions{})
		if err != nil {
			return err
		}

		// Update the original pod reference with the latest state
		*pod = *updatedPod
		return nil
	})
}

func (m *Manager) OpenInteractiveShell(ctx context.Context, pod *corev1.Pod, shell string) error {
	req := m.client().ClientSet().CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(pod.Namespace).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Container: Namespace,
		Command:   []string{shell},
		Stdin:     true,
		Stdout:    true,
		Stderr:    true,
		TTY:       true,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(m.client().Config(), "POST", req.URL())
	if err != nil {
		return err
	}

	// Set up terminal for raw mode
	oldState, err := setupTerminal()
	if err != nil {
		return fmt.Errorf("failed to setup terminal: %w", err)
	}
	defer func() {
		_ = restoreTerminal(oldState)
	}()

	return exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Tty:    true,
	})
}

// PrintPodSpecYAML builds the pod spec and prints it as YAML without creating anything.
func (m *Manager) PrintPodSpecYAML(opts cmdoptions.ExecOptions, isDaemon bool) error {
	var podSpec corev1.PodSpec
	var err error
	if isDaemon {
		podSpec, err = buildDaemonPodSpec(opts)
	} else {
		podSpec, err = buildPodSpec(opts, false)
	}
	if err != nil {
		return err
	}

	pod := corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%v-", Namespace),
			Namespace:    Namespace,
			Labels: map[string]string{
				"host": m.myHostname,
			},
			Annotations: maps.Clone(opts.Annotations),
		},
		Spec: podSpec,
	}
	maps.Copy(pod.Labels, opts.Labels)

	data, err := json.Marshal(pod)
	if err != nil {
		return fmt.Errorf("failed to marshal pod spec: %w", err)
	}
	yamlData, err := yaml.JSONToYAML(data)
	if err != nil {
		return fmt.Errorf("failed to convert to YAML: %w", err)
	}

	fmt.Println("---")
	fmt.Print(string(yamlData))
	return nil
}

// DaemonInfo holds information about a daemon pod for display.
type DaemonInfo struct {
	Name      string
	PodName   string
	Phase     corev1.PodPhase
	Node      string
	Age       time.Duration
	Image     string
	HelmFound bool
	IsHelm4   bool
	HomeDir   string
}

// ListDaemonPods returns information about all daemon pods in the namespace.
func (m *Manager) ListDaemonPods() ([]DaemonInfo, error) {
	pods, err := m.client().ClientSet().CoreV1().Pods(Namespace).List(m.ctx, metav1.ListOptions{
		LabelSelector: "daemon",
	})
	if err != nil {
		return nil, err
	}

	var infos []DaemonInfo
	for _, pod := range pods.Items {
		daemonName := pod.Labels["daemon"]
		if daemonName == "" {
			continue
		}
		info := DaemonInfo{
			Name:    daemonName,
			PodName: pod.Name,
			Phase:   pod.Status.Phase,
			Node:    pod.Spec.NodeName,
			Image:   pod.Spec.Containers[0].Image,
		}
		if pod.Status.StartTime != nil {
			info.Age = time.Since(pod.Status.StartTime.Time)
		}
		info.HelmFound = pod.Annotations[hipconsts.AnnotationHelmFound] == "true"
		info.IsHelm4 = pod.Annotations[hipconsts.AnnotationHelm4] == "true"
		info.HomeDir = pod.Annotations[hipconsts.AnnotationHomeDirectory]
		infos = append(infos, info)
	}
	return infos, nil
}

// GetDaemonStatus returns detailed status information for a specific daemon pod.
func (m *Manager) GetDaemonStatus(name string) (*DaemonInfo, error) {
	pod, err := m.GetDaemonPod(name)
	if err != nil {
		return nil, err
	}

	info := &DaemonInfo{
		Name:    name,
		PodName: pod.Name,
		Phase:   pod.Status.Phase,
		Node:    pod.Spec.NodeName,
		Image:   pod.Spec.Containers[0].Image,
	}
	if pod.Status.StartTime != nil {
		info.Age = time.Since(pod.Status.StartTime.Time)
	}
	info.HelmFound = pod.Annotations[hipconsts.AnnotationHelmFound] == "true"
	info.IsHelm4 = pod.Annotations[hipconsts.AnnotationHelm4] == "true"
	info.HomeDir = pod.Annotations[hipconsts.AnnotationHomeDirectory]
	return info, nil
}
