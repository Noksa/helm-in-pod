package internal

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"github.com/Noksa/operator-home/pkg/operatorkclient"
	"github.com/fatih/color"
	"github.com/noksa/go-helpers/helpers/gopointer"
	"github.com/noksa/helm-in-pod/internal/cmdoptions"
	"github.com/noksa/helm-in-pod/internal/helmtar"
	"github.com/noksa/helm-in-pod/internal/hipembedded"
	"github.com/noksa/helm-in-pod/internal/logz"
	log "github.com/sirupsen/logrus"
	"go.uber.org/multierr"
	"io"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"os"
	"os/signal"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	"strings"
	"syscall"
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
	pods, err := clientSet.CoreV1().Pods(HelmInPodNamespace).List(context.Background(), opts)
	if err != nil {
		return err
	}
	for _, pod := range pods.Items {
		log.Debugf("%v Removing '%v' pod", logz.LogHost(), pod.Name)
		err = clientSet.CoreV1().Pods(HelmInPodNamespace).Delete(context.Background(), pod.Name, metav1.DeleteOptions{})
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
		Value: strconv.Itoa(int(time.Hour * 2)),
	})
	resourceList := corev1.ResourceList{
		"cpu":    resource.MustParse(opts.Cpu),
		"memory": resource.MustParse(opts.Memory),
	}
	labels := map[string]string{"host": myHostname}
	for k, v := range opts.Labels {
		labels[k] = v
	}
	securityContext := &corev1.SecurityContext{}
	if opts.RunAsUser > -1 {
		securityContext.RunAsUser = gopointer.NewOf(opts.RunAsUser)
	}
	if opts.RunAsGroup > -1 {
		securityContext.RunAsGroup = gopointer.NewOf(opts.RunAsGroup)
	}
	podSpec := corev1.PodSpec{
		Containers: []corev1.Container{{
			Name:            "helm-in-pod",
			ImagePullPolicy: corev1.PullPolicy(opts.PullPolicy),
			Image:           opts.Image,
			Command: []string{
				"sh", "-cue",
			},
			Env: envVars,
			Resources: corev1.ResourceRequirements{
				Requests: resourceList,
				Limits:   resourceList,
			},
			SecurityContext: securityContext,
			Args:            []string{hipembedded.GetShScript()},
			WorkingDir:      "/",
			StartupProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					Exec: &corev1.ExecAction{Command: []string{"" +
						"sh", "-c", "([ -f /tmp/ready ] && exit 0) || exit 1"}},
				},
				TimeoutSeconds:   2,
				PeriodSeconds:    1,
				SuccessThreshold: 1,
				FailureThreshold: 60,
			},
		}},
		RestartPolicy:                 corev1.RestartPolicyNever,
		ServiceAccountName:            HelmInPodNamespace,
		AutomountServiceAccountToken:  gopointer.NewOf(true),
		TerminationGracePeriodSeconds: gopointer.NewOf[int64](300),
	}
	if opts.ImagePullSecret != "" {
		podSpec.ImagePullSecrets = append(podSpec.ImagePullSecrets, corev1.LocalObjectReference{Name: opts.ImagePullSecret})
	}
	pod, err := clientSet.CoreV1().Pods(HelmInPodNamespace).Create(ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{GenerateName: fmt.Sprintf("%v-", HelmInPodNamespace), Labels: labels},
		Spec:       podSpec,
	}, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	// let's check interrupt signals
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		if pod != nil && pod.Name != "" {
			log.Warnf("%v Interrupted! Destroying helm pod", logz.LogHost())
			destroyErr := h.DeleteHelmPods(opts, cmdoptions.PurgeOptions{All: false})
			if destroyErr != nil {
				log.Errorf("Couldn't destroy helm pods: %v", destroyErr.Error())
			}
		}
		<-c
		os.Exit(1) // second signal. Exit directly.
	}()
	log.Debugf("%v %v pod has been created", logz.LogHost(), color.CyanString(pod.Name))
	return pod, h.waitUntilPodIsRunning(pod)
}

func (h *HelmPod) waitUntilPodIsRunning(pod *corev1.Pod) error {
	log.Infof("%v Waiting until %v pod is ready", logz.LogHost(), color.CyanString(pod.Name))
	t := time.Now()
	var mErr error
	mErr = multierr.Append(mErr, fmt.Errorf("timeout waiting pod readiness"))
	for time.Since(t) <= time.Minute*5 {
		stdout, stderr, err := operatorkclient.RunCommandInPod("[ -f /tmp/ready ] && echo ready", HelmInPodNamespace, pod.Name, pod.Namespace, nil)
		if err == nil && strings.Contains(stdout, "ready") {
			mErr = nil
			break
		}
		if err != nil {
			mErr = multierr.Append(mErr, fmt.Errorf(stderr))
			mErr = multierr.Append(mErr, err)
		}
		time.Sleep(time.Second)
	}
	if mErr == nil {
		log.Debugf("%v %v pod is ready", logz.LogHost(), color.CyanString(pod.Name))
	}
	return mErr
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

func (h *HelmPod) StreamLogsFromPod(ctx context.Context, pod *corev1.Pod, writer io.Writer, since time.Time) error {
	req := clientSet.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
		Follow:    true,
		SinceTime: &metav1.Time{Time: since},
	})
	stream, err := req.Stream(ctx)
	if err != nil {
		return err
	}
	defer stream.Close()
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

func (h *HelmPod) GetPodPhase(ctx context.Context, pod *corev1.Pod) (corev1.PodPhase, error) {
	myPod, err := clientSet.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
	if err != nil {
		return corev1.PodFailed, client.IgnoreNotFound(err)
	}
	return myPod.Status.Phase, nil
}
