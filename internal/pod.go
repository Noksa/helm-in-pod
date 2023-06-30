package internal

import (
	"bytes"
	"fmt"
	"github.com/Noksa/operator-home/pkg/operatorkclient"
	"github.com/noksa/helm-in-pod/internal/helmtar"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"os"
	"path/filepath"
	"time"
)

type HelmPod struct {
}

func (h *HelmPod) DeleteAllPods() error {
	pods, err := clientSet.CoreV1().Pods(HelmInPodNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("host=%v", myHostname),
	})
	if err != nil {
		return err
	}
	for _, pod := range pods.Items {
		log.Infof("Removing '%v' pod", pod.Name)
		err = clientSet.CoreV1().Pods(HelmInPodNamespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func (h *HelmPod) CreateHelmPod(opts HelmInPodFlags) (*corev1.Pod, error) {
	err := h.DeleteAllPods()
	if err != nil {
		return nil, err
	}
	log.Infof("Creating '%v' pod", HelmInPodNamespace)
	pod, err := clientSet.CoreV1().Pods(HelmInPodNamespace).Create(ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{GenerateName: fmt.Sprintf("%v-", HelmInPodNamespace), Labels: map[string]string{"host": myHostname}},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{corev1.Container{
				Name:  "helm-in-pod",
				Image: opts.Image,
				Command: []string{
					"sh", "-cuex",
				},
				Args: []string{`
trap 'exit 0' SIGINT SIGTERM
while true; do
  sleep 1
done`},
				WorkingDir: "/",
				StartupProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						Exec: &corev1.ExecAction{Command: []string{"" +
							"sh", "-c", "sleep 1 && exit 0"}},
					},
					TimeoutSeconds:   5,
					PeriodSeconds:    1,
					SuccessThreshold: 1,
					FailureThreshold: 60,
				},
			}},
			RestartPolicy:      "Never",
			ServiceAccountName: HelmInPodNamespace,
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return pod, h.waitUntilPodIsRunning(pod)
}

func (h *HelmPod) waitUntilPodIsRunning(pod *corev1.Pod) error {
	log.Infof("Waiting until '%v' pod is ready", pod.Name)
	f := func() (bool, error) {
		myPod, err := clientSet.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		canContinue := false
		switch myPod.Status.Phase {
		case corev1.PodRunning:
			canContinue = true
		}
		if !canContinue {
			return false, nil
		}
		allReady := true
		for _, container := range myPod.Status.ContainerStatuses {
			if !container.Ready {
				allReady = false
				break
			}
		}
		return allReady, nil
	}
	return wait.PollImmediate(time.Second, time.Second*30, f)
}

func (h *HelmPod) CopyFileToPod(pod *corev1.Pod, srcPath string, destPath string) error {
	buffer := &bytes.Buffer{}
	err := helmtar.Compress(srcPath, destPath, buffer)
	if err != nil {
		return err
	}

	// Create a stream to the container
	req := clientSet.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(pod.Namespace).
		SubResource("exec").
		Param("container", HelmInPodNamespace)

	dir := filepath.Dir(destPath)

	req.VersionedParams(&corev1.PodExecOptions{
		Container: HelmInPodNamespace,
		Command: []string{"sh", "-ceu", fmt.Sprintf(`
mkdir -p %v
tar zxf - -C %v`, dir, filepath.Dir(dir))},
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(operatorkclient.GetClientConfig(), "POST", req.URL())
	if err != nil {
		return err
	}

	// Create a stream to the container
	log.Infof("Copying '%v' file to '%v' pod in '%v' path", srcPath, pod.Name, destPath)
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  bytes.NewReader(buffer.Bytes()),
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Tty:    false,
	})
	return err
}
