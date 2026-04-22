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
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Noksa/operator-home/pkg/operatorkclient"
	"github.com/fatih/color"
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/noksa/helm-in-pod/internal/cmdoptions"
	"github.com/noksa/helm-in-pod/internal/helmtar"
	"github.com/noksa/helm-in-pod/internal/hipconsts"
	"github.com/noksa/helm-in-pod/internal/hipretry"
	"github.com/noksa/helm-in-pod/internal/logz"
)

const Namespace = "helm-in-pod"

type Manager struct {
	ctx          context.Context
	myHostname   string
	interrupted  atomic.Bool
	invocationID string // unique per process; prevents concurrent instances from deleting each other's pods
}

func NewManager(ctx context.Context, hostname string) *Manager {
	return &Manager{
		ctx:          ctx,
		myHostname:   hostname,
		invocationID: uuid.New().String(),
	}
}
func (m *Manager) client() *operatorkclient.Client {
	return operatorkclient.DefaultClient()
}

func (m *Manager) DeleteHelmPods(execOptions cmdoptions.ExecOptions, purgeOptions cmdoptions.PurgeOptions) error {
	opts := metav1.ListOptions{}
	if !purgeOptions.All {
		// Include the per-process operation ID so each process only deletes its own pods.
		// Without this, concurrent instances on the same host would share the
		// "host=<hostname>" selector and delete each other's pods on startup.
		selector := fmt.Sprintf("host=%v,%v=%v", m.myHostname, hipconsts.LabelOperationID, m.invocationID)
		for k, v := range execOptions.Labels {
			selector = fmt.Sprintf("%v,%v=%v", selector, k, v)
		}
		opts.LabelSelector = selector
	}
	pods, err := m.client().ClientSet().CoreV1().Pods(Namespace).List(m.ctx, opts)
	if err != nil {
		return err
	}
	for i := range pods.Items {
		pod := &pods.Items[i]
		logz.Host().Debug().Msgf("Deleting '%v' pod", pod.Name)

		// Extract operation ID from pod labels and delete associated PDB
		if operationID, ok := pod.Labels[hipconsts.LabelOperationID]; ok {
			if err := m.DeletePodDisruptionBudgets(m.ctx, operationID); err != nil {
				logz.Host().Warn().Msgf("Failed to delete PodDisruptionBudget for operation %s: %v", operationID, err)
			}
		}

		// Only force-delete pods that have already terminated. For pods still
		// in Running/Pending phase, respect the default grace period so the
		// kubelet can complete container shutdown and avoid orphaned containers.
		deleteOpts := metav1.DeleteOptions{}
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			zero := int64(0)
			deleteOpts.GracePeriodSeconds = &zero
		}

		err = m.client().ClientSet().CoreV1().Pods(Namespace).Delete(m.ctx, pod.Name, deleteOpts)
		if err != nil {
			return err
		}
		logz.Host().Debug().Msgf("'%v' pod has been deleted", pod.Name)
	}
	return nil
}

func (m *Manager) CreateHelmPod(opts cmdoptions.ExecOptions) (*corev1.Pod, error) {
	err := m.DeleteHelmPods(opts, cmdoptions.PurgeOptions{All: false})
	if err != nil {
		return nil, err
	}
	logz.Host().Info().Msgf("Creating '%v' pod", color.MagentaString(Namespace))

	podSpec, err := buildPodSpec(opts, false)
	if err != nil {
		return nil, err
	}

	labels := map[string]string{
		"host":                     m.myHostname,
		hipconsts.LabelOperationID: m.invocationID,
		hipconsts.LabelManagedBy:   Namespace,
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
		if err := m.CreatePodDisruptionBudget(m.ctx, m.invocationID); err != nil {
			// If PDB creation fails, clean up the pod immediately
			zero := int64(0)
			_ = m.client().ClientSet().CoreV1().Pods(Namespace).Delete(m.ctx, pod.Name, metav1.DeleteOptions{
				GracePeriodSeconds: &zero,
			})
			return nil, fmt.Errorf("failed to create PodDisruptionBudget: %w", err)
		}
	}

	// Handle interrupt signals. done is closed when CreateHelmPod returns so
	// the goroutine exits promptly and does not leak across invocations.
	done := make(chan struct{})
	defer close(done)

	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(c)

	go func() {
		select {
		case <-done:
			return
		case <-c:
		}
		if pod != nil && pod.Name != "" {
			logz.Host().Warn().Msg("Interrupted! Destroying helm pod")
			destroyErr := m.DeleteHelmPods(opts, cmdoptions.PurgeOptions{All: false})
			if destroyErr != nil {
				logz.Host().Error().Msgf("Couldn't destroy helm pods: %v", destroyErr.Error())
			}
			// Clean up PDB if it was created
			if opts.CreatePDB {
				_ = m.DeletePodDisruptionBudgets(m.ctx, m.invocationID)
			}
			m.interrupted.Store(true)
		}
		select {
		case <-done:
			return
		case <-c:
			os.Exit(1)
		}
	}()

	logz.Host().Debug().Msgf("%v pod has been created", color.MagentaString(pod.Name))
	return pod, m.waitUntilPodIsRunning(pod)
}

func (m *Manager) waitUntilPodIsRunning(pod *corev1.Pod) error {
	logz.Host().Info().Msgf("Waiting until %v pod is ready", color.MagentaString(pod.Name))

	err := wait.PollUntilContextTimeout(m.ctx, time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		if m.interrupted.Load() {
			return false, fmt.Errorf("interrupted while was waiting for pod readiness")
		}

		stdout, stderr, err := m.client().ExecInPod("[ -f /tmp/ready ] && echo ready", Namespace, pod.Name, pod.Namespace)
		if err != nil {
			logz.Pod().Debug().Msgf("Not ready yet: %v %v", stderr, err.Error())
			return false, nil
		}
		if strings.Contains(stdout, "ready") {
			logz.Host().Debug().Msgf("%v pod is ready", color.CyanString(pod.Name))
			return true, nil
		}
		return false, nil
	})
	if wait.Interrupted(err) {
		return fmt.Errorf("timeout waiting pod readiness")
	}
	return err
}

func (m *Manager) waitUntilPodIsDeleted(podName string) error {
	logz.Host().Debug().Msgf("Waiting for pod %v to be deleted", color.CyanString(podName))

	err := wait.PollUntilContextTimeout(m.ctx, time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		if m.interrupted.Load() {
			return false, fmt.Errorf("interrupted while waiting for pod deletion")
		}

		_, getErr := m.client().ClientSet().CoreV1().Pods(Namespace).Get(ctx, podName, metav1.GetOptions{})
		if getErr != nil {
			if k8serrors.IsNotFound(getErr) {
				logz.Host().Info().Msgf("Pod %v has been deleted", color.CyanString(podName))
				return true, nil
			}
			return false, fmt.Errorf("error checking pod status: %w", getErr)
		}
		return false, nil
	})
	if wait.Interrupted(err) {
		return fmt.Errorf("timeout waiting for pod deletion")
	}
	return err
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
	cmd := fmt.Sprintf("mkdir -p %s && tar zxf - -C /", dir)

	return hipretry.Retry(attempts, func() error {
		logz.HostPod().Info().Msgf("Copying %v to %v", color.CyanString(srcPath), color.MagentaString(destPath))

		_, stderr, err := m.client().ExecInPod(cmd, Namespace, pod.Name, pod.Namespace,
			operatorkclient.WithContext(m.ctx),
			operatorkclient.WithTimeout(time.Minute*10),
			operatorkclient.WithStdin(bytes.NewReader(buffer.Bytes())),
		)
		if err != nil {
			return fmt.Errorf("%w: %s", err, stderr)
		}

		logz.HostPod().Debug().Msgf("%v has been copied to %v", color.CyanString(srcPath), color.MagentaString(destPath))
		return nil
	})
}

// isPodPathRegularFile checks whether podPath is a regular file inside the pod.
func (m *Manager) isPodPathRegularFile(pod *corev1.Pod, podPath string) bool {
	cmd := fmt.Sprintf("test -f %s", podPath)
	_, _, err := m.client().ExecInPod(cmd, Namespace, pod.Name, pod.Namespace,
		operatorkclient.WithRawCommand(true))
	return err == nil
}

// CopyFileFromPod copies a file or directory from the pod to the host.
//
// For regular files:
//   - If hostPath is "." or an existing directory, the file is placed inside it
//     keeping its original name (like cp).
//   - Otherwise hostPath is treated as the destination file path.
//
// For directories, the contents are placed directly inside hostPath.
func (m *Manager) CopyFileFromPod(pod *corev1.Pod, podPath string, hostPath string, attempts int) error {
	podPath = filepath.Clean(podPath)
	hostPath = filepath.Clean(hostPath)

	return hipretry.Retry(attempts, func() error {
		isFile := m.isPodPathRegularFile(pod, podPath)

		var tarCmd string
		var extractDir string

		if isFile {
			// Determine whether hostPath is a directory target or a file target.
			hostIsDir := hostPath == "." || isLocalDir(hostPath)
			if hostIsDir {
				// Place file inside the directory, keeping its original name.
				tarCmd = fmt.Sprintf("tar czf - -C %s %s", filepath.Dir(podPath), filepath.Base(podPath))
				extractDir = hostPath
			} else {
				// hostPath is the target file path.
				tarCmd = fmt.Sprintf("tar czf - -C %s %s", filepath.Dir(podPath), filepath.Base(podPath))
				extractDir = filepath.Dir(hostPath)
			}
		} else {
			tarCmd = fmt.Sprintf("tar czf - -C %s .", podPath)
			extractDir = hostPath
		}

		logz.HostPod().Info().Msgf("Copying %v to %v", color.MagentaString(podPath), color.CyanString(hostPath))

		var stdout bytes.Buffer
		_, _, err := m.client().ExecInPod(tarCmd, Namespace, pod.Name, pod.Namespace,
			operatorkclient.WithContext(m.ctx),
			operatorkclient.WithTimeout(time.Minute*10),
			operatorkclient.WithRawCommand(true),
			operatorkclient.WithStdout(&stdout),
		)
		if err != nil {
			return err
		}

		if err := extractTarGz(&stdout, extractDir); err != nil {
			return fmt.Errorf("failed to extract archive: %w", err)
		}

		// For file targets (not directory targets), rename if the pod filename
		// differs from the desired host filename.
		if isFile && !isLocalDir(hostPath) && hostPath != "." {
			if filepath.Base(podPath) != filepath.Base(hostPath) {
				extracted := filepath.Join(extractDir, filepath.Base(podPath))
				if renameErr := os.Rename(extracted, hostPath); renameErr != nil {
					return fmt.Errorf("failed to rename extracted file: %w", renameErr)
				}
			}
		}

		logz.HostPod().Debug().Msgf("%v has been copied to %v", color.MagentaString(podPath), color.CyanString(hostPath))
		return nil
	})
}

// isLocalDir returns true if path exists and is a directory on the local filesystem.
func isLocalDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
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
		logz.Host().Info().Msgf("Force flag enabled, recreating daemon pod %v", color.CyanString(opts.Name))
		if err := m.DeleteDaemonPod(opts.Name); err != nil {
			return nil, fmt.Errorf("failed to delete existing daemon pod: %w", err)
		}
	}

	logz.Host().Info().Msgf("Creating daemon pod '%v'", opts.Name)

	podSpec, err := buildDaemonPodSpec(opts.ExecOptions)
	if err != nil {
		return nil, err
	}

	labels := map[string]string{
		"daemon":                   opts.Name,
		hipconsts.LabelOperationID: m.invocationID,
		hipconsts.LabelManagedBy:   Namespace,
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
		if err := m.CreatePodDisruptionBudget(m.ctx, m.invocationID); err != nil {
			// If PDB creation fails, clean up the pod immediately
			zero := int64(0)
			_ = m.client().ClientSet().CoreV1().Pods(Namespace).Delete(m.ctx, pod.Name, metav1.DeleteOptions{
				GracePeriodSeconds: &zero,
			})
			return nil, fmt.Errorf("failed to create PodDisruptionBudget: %w", err)
		}
	}

	logz.Host().Debug().Msgf("Daemon pod %v has been created", pod.Name)
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
	logz.Host().Info().Msgf("Deleting daemon pod %v", color.CyanString(podName))

	// Get the pod to extract operation ID before deletion
	pod, err := m.client().ClientSet().CoreV1().Pods(Namespace).Get(m.ctx, podName, metav1.GetOptions{})
	if err == nil {
		// Extract operation ID from pod labels and delete associated PDB
		if operationID, ok := pod.Labels[hipconsts.LabelOperationID]; ok {
			if err := m.DeletePodDisruptionBudgets(m.ctx, operationID); err != nil {
				logz.Host().Warn().Msgf("Failed to delete PodDisruptionBudget for operation %s: %v", operationID, err)
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
	// Set up terminal for raw mode
	oldState, err := setupTerminal()
	if err != nil {
		return fmt.Errorf("failed to setup terminal: %w", err)
	}
	defer func() {
		_ = restoreTerminal(oldState)
	}()

	_, _, err = m.client().ExecInPod(shell, Namespace, pod.Name, pod.Namespace,
		operatorkclient.WithContext(ctx),
		operatorkclient.WithTTY(true),
		operatorkclient.WithRawCommand(true),
		operatorkclient.WithStdin(os.Stdin),
		operatorkclient.WithStdout(os.Stdout),
		operatorkclient.WithStderr(os.Stderr),
	)
	return err
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
				"host":                   m.myHostname,
				hipconsts.LabelManagedBy: Namespace,
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
	for i := range pods.Items {
		pod := &pods.Items[i]
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
