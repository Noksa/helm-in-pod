package internal

import (
	"bytes"
	"context"
	"fmt"
	"github.com/Noksa/operator-home/pkg/operatorkclient"
	"github.com/fatih/color"
	"github.com/noksa/helm-in-pod/internal/cmdoptions"
	"github.com/noksa/helm-in-pod/internal/helmtar"
	"github.com/noksa/helm-in-pod/internal/hipembedded"
	"github.com/noksa/helm-in-pod/internal/logz"
	log "github.com/sirupsen/logrus"
	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type HelmPod struct {
}

func (h *HelmPod) DeleteHelmPods(execOptions cmdoptions.ExecOptions, purgeOptions cmdoptions.PurgeOptions) error {
	opts := metav1.ListOptions{}
	if !purgeOptions.All {
		selector := fmt.Sprintf("host=%v", myHostname)
		for k, v := range execOptions.Labels {
			selector = fmt.Sprintf("%v,%v=%v", selector, k, v)
		}
		selector = strings.TrimSuffix(selector, ",")
		selector = strings.TrimPrefix(selector, ",")
		opts.LabelSelector = selector
	}
	pods, err := clientSet.CoreV1().Pods(HelmInPodNamespace).List(ctx, opts)
	if err != nil {
		return err
	}
	for _, pod := range pods.Items {
		log.Infof("%v Removing '%v' pod", logz.LogHost(), pod.Name)
		err = clientSet.CoreV1().Pods(HelmInPodNamespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func (h *HelmPod) CreateHelmPod(opts cmdoptions.ExecOptions) (*corev1.Pod, error) {
	err := h.DeleteHelmPods(opts, cmdoptions.PurgeOptions{All: false})
	if err != nil {
		return nil, err
	}
	log.Infof("%v Creating '%v' pod", logz.LogHost(), HelmInPodNamespace)

	var envVars []corev1.EnvVar
	for _, env := range opts.SubstEnv {
		val := os.Getenv(env)
		envVar := corev1.EnvVar{
			Name:  env,
			Value: val,
		}
		envVars = append(envVars, envVar)
	}
	for k, v := range opts.Env {
		envVar := corev1.EnvVar{
			Name:  k,
			Value: v,
		}
		envVars = append(envVars, envVar)
	}
	envVars = append(envVars, corev1.EnvVar{
		Name:  "TIMEOUT",
		Value: strconv.Itoa(int(opts.Timeout.Seconds())),
	})
	resourceList := corev1.ResourceList{
		"cpu":    resource.MustParse(opts.Cpu),
		"memory": resource.MustParse(opts.Memory),
	}
	labels := map[string]string{"host": myHostname}
	for k, v := range opts.Labels {
		labels[k] = v
	}
	pod, err := clientSet.CoreV1().Pods(HelmInPodNamespace).Create(ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{GenerateName: fmt.Sprintf("%v-", HelmInPodNamespace), Labels: labels},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "helm-in-pod",
				Image: opts.Image,
				Command: []string{
					"sh", "-cue",
				},
				Env: envVars,
				Resources: corev1.ResourceRequirements{
					Requests: resourceList,
					Limits:   resourceList,
				},
				Args:       []string{hipembedded.GetShScript()},
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
	log.Debugf("%v %v pod has been created", logz.LogHost(), color.CyanString(pod.Name))
	return pod, h.waitUntilPodIsRunning(pod)
}

func (h *HelmPod) waitUntilPodIsRunning(pod *corev1.Pod) error {
	log.Infof("%v Waiting until %v pod is ready", logz.LogHost(), color.CyanString(pod.Name))
	f := func(ctx context.Context) (done bool, err error) {
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
	err := wait.PollUntilContextTimeout(ctx, time.Second, time.Second*30, true, f)
	if err == nil {
		log.Debugf("%v %v pod is ready", logz.LogHost(), color.CyanString(pod.Name))
	}
	return err
}

func (h *HelmPod) CopyFileToPod(pod *corev1.Pod, srcPath string, destPath string) error {
	buffer := &bytes.Buffer{}
	srcPath = filepath.Clean(srcPath)
	destPath = filepath.Clean(destPath)
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
tar zxf - -C /`, dir)},
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(operatorkclient.GetClientConfig(), "POST", req.URL())
	if err != nil {
		return err
	}

	// Create a stream to the container
	log.Infof("%v %v Copying %v to %v", logz.LogHost(), logz.LogPod(), color.CyanString(srcPath), color.MagentaString(destPath))
	b := &strings.Builder{}
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  bytes.NewReader(buffer.Bytes()),
		Stdout: b,
		Stderr: b,
		Tty:    false,
	})
	if err != nil {
		err = multierr.Append(err, fmt.Errorf(b.String()))
	}
	if err == nil {
		log.Debugf("%v %v %v has been copied to %v", logz.LogHost(), logz.LogPod(), color.CyanString(srcPath), color.MagentaString(destPath))
	}
	return err
}
