package hippod

import (
	"bufio"
	"bytes"
	"context"
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
	"github.com/noksa/helm-in-pod/internal/hipretry"
	"github.com/noksa/helm-in-pod/internal/logz"
	log "github.com/sirupsen/logrus"
	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const Namespace = "helm-in-pod"

type Manager struct {
	clientSet   *kubernetes.Clientset
	ctx         context.Context
	myHostname  string
	interrupted bool
}

func NewManager(clientSet *kubernetes.Clientset, ctx context.Context, hostname string) *Manager {
	return &Manager{
		clientSet:  clientSet,
		ctx:        ctx,
		myHostname: hostname,
	}
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
	pods, err := m.clientSet.CoreV1().Pods(Namespace).List(context.Background(), opts)
	if err != nil {
		return err
	}
	for _, pod := range pods.Items {
		log.Debugf("%v Deleting '%v' pod", logz.LogHost(), pod.Name)
		err = m.clientSet.CoreV1().Pods(Namespace).Delete(context.Background(), pod.Name, metav1.DeleteOptions{})
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

	podSpec, err := buildPodSpec(opts)
	if err != nil {
		return nil, err
	}

	labels := map[string]string{"host": m.myHostname}
	maps.Copy(labels, opts.Labels)
	annotations := map[string]string{}
	maps.Copy(annotations, opts.Annotations)

	pod, err := m.clientSet.CoreV1().Pods(Namespace).Create(m.ctx, &corev1.Pod{
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
		stdout, stderr, err := operatorkclient.RunCommandInPod("[ -f /tmp/ready ] && echo ready", Namespace, pod.Name, pod.Namespace, nil)
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

func (m *Manager) CopyFileToPod(pod *corev1.Pod, srcPath string, destPath string, attempts int) error {
	buffer := &bytes.Buffer{}
	srcPath = filepath.Clean(srcPath)
	destPath = filepath.Clean(destPath)
	err := helmtar.Compress(srcPath, destPath, buffer)
	if err != nil {
		return err
	}

	dir := filepath.Dir(destPath)
	req := m.clientSet.CoreV1().RESTClient().Post().
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
		exec, err := remotecommand.NewSPDYExecutor(operatorkclient.GetClientConfig(), "POST", req.URL())
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

func (m *Manager) StreamLogsFromPod(ctx context.Context, pod *corev1.Pod, writer io.Writer, since time.Time) error {
	req := m.clientSet.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
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
	myPod, err := m.clientSet.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
	if err != nil {
		return corev1.PodFailed, client.IgnoreNotFound(err)
	}
	return myPod.Status.Phase, nil
}
