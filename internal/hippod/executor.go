package hippod

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Noksa/operator-home/pkg/operatorkclient"
	"github.com/fatih/color"
	"github.com/noksa/helm-in-pod/internal/cmdoptions"
	"github.com/noksa/helm-in-pod/internal/hipconsts"
	"github.com/noksa/helm-in-pod/internal/hiperrors"
	"github.com/noksa/helm-in-pod/internal/hipretry"
	"github.com/noksa/helm-in-pod/internal/logz"
	"helm.sh/helm/v4/pkg/cli"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type UserInfo struct {
	HomeDirectory string
	Whoami        string
	ID            string
}

func (m *Manager) GetPodUserInfo(pod *corev1.Pod) (*UserInfo, error) {
	var stdout string

	err := hipretry.Retry(3, func() error {
		logz.Pod().Debug().Msg("Determining user home directory")
		var stderr string
		var err error
		stdout, stderr, err = m.client().RunCommandInPod(
			`echo "${HOME}:::$(whoami):::$(id)"`,
			Namespace, pod.Name, pod.Namespace, nil)
		if err != nil {
			return fmt.Errorf("%s: %w", stderr, err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	stdout = strings.TrimSpace(stdout)
	splitted := strings.Split(stdout, ":::")
	homeDirectory := strings.TrimSuffix(splitted[0], "/")
	whoami := "unknown"
	id := "unknown"
	if len(splitted) >= 2 {
		whoami = splitted[1]
	}
	if len(splitted) >= 3 {
		id = splitted[2]
	}

	if homeDirectory == "" {
		userInfo := fmt.Sprintf("id: %v, whoami: %v", color.GreenString(id), color.YellowString(whoami))
		return nil, fmt.Errorf("user (%v) in the image doesn't have home directory", userInfo)
	}

	logz.Pod().Debug().Msgf("(%v) home directory: %v", color.GreenString(whoami), color.MagentaString(homeDirectory))
	return &UserInfo{
		HomeDirectory: homeDirectory,
		Whoami:        whoami,
		ID:            id,
	}, nil
}

func (m *Manager) SyncHelmRepositories(pod *corev1.Pod, opts cmdoptions.ExecOptions, homeDirectory string, isHelm4 bool) error {
	settings := cli.New()
	_, statErr := os.Stat(settings.RepositoryConfig)
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return statErr
	}
	if statErr != nil {
		return nil
	}

	err := hipretry.Retry(opts.CopyAttempts, func() error {
		logz.Pod().Debug().Msgf("Creating %v/.config/helm directory", homeDirectory)
		_, stderr, err := m.client().RunCommandInPod(
			`set +e; mkdir -p "${HOME}/.config/helm" &>/dev/null`,
			Namespace, pod.Name, pod.Namespace, nil)
		if err != nil {
			return fmt.Errorf("%s: %w", stderr, err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	err = m.CopyFileToPod(pod, settings.RepositoryConfig,
		fmt.Sprintf("%v/.config/helm/repositories.yaml", homeDirectory), opts.CopyAttempts)
	if err != nil {
		return err
	}

	return m.updateHelmRepositories(pod, opts, isHelm4)
}

func (m *Manager) UpdateHelmRepositories(pod *corev1.Pod, opts cmdoptions.ExecOptions, isHelm4 bool) error {
	err := m.updateHelmRepositories(pod, opts, isHelm4)
	if err != nil {
		return err
	}

	// Add annotation with last update time in RFC3339 format
	updateTime := time.Now().Format(time.RFC3339)
	return m.AnnotatePod(pod, map[string]string{
		hipconsts.AnnotationLastRepoUpdateTime: updateTime,
	})
}

func (m *Manager) updateHelmRepositories(pod *corev1.Pod, opts cmdoptions.ExecOptions, isHelm4 bool) error {
	if len(opts.UpdateRepo) == 0 {
		return hipretry.Retry(opts.UpdateRepoAttempts, func() error {
			logz.Pod().Info().Msgf("Fetching updates from %v helm repositories", color.GreenString("all"))
			cmdToUse := "helm repo update"
			if !isHelm4 {
				cmdToUse = fmt.Sprintf("%v --fail-on-repo-update-fail", cmdToUse)
			}
			stdout, stderr, err := m.client().RunCommandInPod(cmdToUse,
				Namespace, pod.Name, pod.Namespace, nil)
			if err != nil {
				return fmt.Errorf("%w\n%v\n%v", err, stdout, stderr)
			}
			logz.Pod().Debug().Msg("Helm repository updates have been fetched")
			return nil
		})
	}

	var errs []error
	for _, repo := range opts.UpdateRepo {
		err := hipretry.Retry(opts.UpdateRepoAttempts, func() error {
			logz.Pod().Info().Msgf("Fetching updates from %v helm repository", color.CyanString(repo))
			cmdToUse := fmt.Sprintf("helm repo update %v", repo)
			if !isHelm4 {
				cmdToUse = fmt.Sprintf("%v --fail-on-repo-update-fail", cmdToUse)
			}
			stdout, stderr, err := m.client().RunCommandInPod(cmdToUse,
				Namespace, pod.Name, pod.Namespace, nil)
			if err != nil {
				return fmt.Errorf("%w\n%v\n%v", err, stdout, stderr)
			}
			logz.Pod().Debug().Msgf("%v helm repository updates have been fetched", color.CyanString(repo))
			return nil
		})
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (m *Manager) CopyUserFiles(pod *corev1.Pod, opts cmdoptions.ExecOptions, expandPath func(string) (string, error), cleanPaths []string) error {
	// Delete specified paths first to ensure clean state
	if len(cleanPaths) > 0 {
		cmd := fmt.Sprintf("rm -rf %s", strings.Join(cleanPaths, " "))
		logz.Pod().Debug().Msgf("Cleaning up files: %v", cmd)
		stdOut, stdErr, err := m.client().RunCommandInPod(cmd, Namespace, pod.Name, pod.Namespace, nil)
		if err != nil {
			return fmt.Errorf("%v\n%v\n%v", err, stdErr, stdOut)
		}
	}

	for k, v := range opts.FilesAsMap {
		path, err := expandPath(k)
		if err != nil {
			return err
		}
		err = m.CopyFileToPod(pod, path, v, opts.CopyAttempts)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) ExecuteCommand(ctx context.Context, pod *corev1.Pod, command string, homeDirectory string, opts cmdoptions.ExecOptions) error {
	copyFromMode := len(opts.CopyFrom) > 0

	scriptPath := fmt.Sprintf("%v/wrapped-script.sh", homeDirectory)

	tempScriptFile, err := os.CreateTemp("", hipconsts.HelmInPodNamespace)
	if err != nil {
		return err
	}
	defer func() {
		_ = tempScriptFile.Close()
		_ = os.RemoveAll(tempScriptFile.Name())
	}()

	err = os.Chmod(tempScriptFile.Name(), os.ModePerm)
	if err != nil {
		return err
	}

	_, err = tempScriptFile.WriteString("#!/bin/sh\nset -eu\n")
	if err != nil {
		return err
	}
	_, err = tempScriptFile.WriteString(command)
	if err != nil {
		return err
	}
	_, err = tempScriptFile.WriteString("\n")
	if err != nil {
		return err
	}

	since := time.Now()
	err = m.CopyFileToPod(pod, tempScriptFile.Name(), scriptPath, opts.CopyAttempts)
	if err != nil {
		return err
	}

	logz.Pod().Info().Msgf("Running '%v' command", color.YellowString(command))

	b := &bytes.Buffer{}
	var logWriter io.Writer

	// In copy-from mode, wrap the writer to intercept the exit code marker.
	// We use a cancellable context so the marker writer can break the blocking
	// StreamLogsFromPod call once the marker is detected.
	var mw *exitCodeMarkerWriter
	var cancelStream context.CancelFunc
	streamCtx := context.Background()
	if copyFromMode {
		streamCtx, cancelStream = context.WithCancel(streamCtx)
		defer cancelStream()
		mw = newExitCodeMarkerWriter(io.MultiWriter(os.Stdout, b), cancelStream)
		logWriter = mw
	} else {
		logWriter = io.MultiWriter(os.Stdout, b)
	}

	go func() {
		<-ctx.Done()
		logz.Host().Warn().Msg("Timed out!")
		for {
			_, _, err := m.client().RunCommandInPod("kill -term 1",
				hipconsts.HelmInPodNamespace, pod.Name, pod.Namespace, nil)
			if err == nil {
				return
			}
			time.Sleep(time.Millisecond * 50)
		}
	}()

	wg := sync.WaitGroup{}
	wg.Go(func() {
		for {
			phase, err := m.GetPodPhase(context.Background(), pod)
			if err != nil {
				if client.IgnoreNotFound(err) == nil {
					return
				}
				time.Sleep(time.Millisecond * 25)
				continue
			}
			if phase == corev1.PodFailed || phase == corev1.PodSucceeded {
				_ = m.StreamLogsFromPod(context.Background(), pod, logWriter, since)
				return
			}
			err = m.StreamLogsFromPod(streamCtx, pod, logWriter, since)
			since = time.Now()
			if err == nil {
				return
			}
			if copyFromMode && mw.Found() {
				return
			}
			logz.Host().Info().Msgf("got an error from streaming pod logs: %v", err)
			time.Sleep(time.Millisecond * 25)
		}
	})
	wg.Wait()

	if copyFromMode && mw.Found() {
		code := mw.ExitCode()
		logz.Pod().Info().Msgf("Command exited with code %d (pod kept alive for copy-from)", code)
		if code != 0 {
			return &hiperrors.ExitCodeError{Code: int32(code)}
		}
		return nil
	}
	return m.waitForPodCompletion(ctx, pod)
}

func (m *Manager) ExecuteCommandInDaemon(ctx context.Context, pod *corev1.Pod, command string, homeDirectory string, timeout time.Duration, opts cmdoptions.ExecOptions) error {
	scriptPath := fmt.Sprintf("%v/wrapped-script.sh", homeDirectory)

	tempScriptFile, err := os.CreateTemp("", hipconsts.HelmInPodNamespace)
	if err != nil {
		return err
	}
	defer func() {
		_ = tempScriptFile.Close()
		_ = os.RemoveAll(tempScriptFile.Name())
	}()

	err = os.Chmod(tempScriptFile.Name(), os.ModePerm)
	if err != nil {
		return err
	}

	_, err = tempScriptFile.WriteString("set -eu\n")
	if err != nil {
		return err
	}

	// Log command execution to PID 1 stdout
	_, err = fmt.Fprintf(tempScriptFile, "echo \"[$(date +%%D-%%T)] Executing: %s\" > /proc/1/fd/1\n", command)
	if err != nil {
		return err
	}

	// Export environment variables
	for _, env := range opts.SubstEnv {
		val := os.Getenv(env)
		_, err = fmt.Fprintf(tempScriptFile, "export %s=%q\n", env, val)
		if err != nil {
			return err
		}
	}
	for k, v := range opts.Env {
		_, err = fmt.Fprintf(tempScriptFile, "export %s=%q\n", k, v)
		if err != nil {
			return err
		}
	}

	_, err = tempScriptFile.WriteString(command)
	if err != nil {
		return err
	}

	// Log command completion to PID 1 stdout
	_, err = tempScriptFile.WriteString("\necho \"[$(date +%D-%T)] Executed successfully\" > /proc/1/fd/1\n")
	if err != nil {
		return err
	}

	err = m.CopyFileToPod(pod, tempScriptFile.Name(), scriptPath, 3)
	if err != nil {
		return err
	}

	logz.Pod().Info().Msgf("Running '%v' command", color.YellowString(command))

	runOpts := operatorkclient.RunCommandInPodOptions{
		Context:       ctx,
		Timeout:       timeout,
		Command:       fmt.Sprintf("sh %s", scriptPath),
		PodName:       pod.Name,
		PodNamespace:  pod.Namespace,
		ContainerName: Namespace,
		Stdout:        os.Stdout,
		Stderr:        os.Stderr,
	}

	_, _, err = m.client().RunCommandInPodWithOptions(runOpts)
	if err != nil {
		if code := parseExitCodeFromError(err); code != hiperrors.ExitCodeUnknown {
			logz.Pod().Info().Msgf("Command exited with code %d", code)
			return &hiperrors.ExitCodeError{Code: int32(code)}
		}
	}
	return err
}

func (m *Manager) waitForPodCompletion(ctx context.Context, pod *corev1.Pod) error {
	logz.Host().Debug().Msg("Waiting 60s until pod phase is changed to failed/succeeded")

	timeout := time.Second * 60
	start := time.Now()
	var phase corev1.PodPhase

	for time.Since(start) <= timeout {
		var err error
		phase, err = m.GetPodPhase(context.Background(), pod)
		if err == nil && phase != corev1.PodRunning {
			break
		}
		time.Sleep(time.Millisecond * 100)
	}

	logz.Host().Debug().Msgf("Pod got phase: %v", color.CyanString("%v", phase))

	if phase == corev1.PodFailed {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// Extract the actual exit code from the container status
		exitCode := m.getContainerExitCode(pod)
		if exitCode != hiperrors.ExitCodeUnknown {
			logz.Pod().Info().Msgf("Command exited with code %d", exitCode)
			return &hiperrors.ExitCodeError{Code: exitCode}
		}
		return fmt.Errorf("pod failed")
	}
	if phase == corev1.PodSucceeded {
		return nil
	}
	return fmt.Errorf("unexpected pod phase: %v", phase)
}

// getContainerExitCode retrieves the exit code from the pod's container status.
// Returns ExitCodeUnknown if the exit code cannot be determined.
func (m *Manager) getContainerExitCode(pod *corev1.Pod) int32 {
	myPod, err := m.client().ClientSet().CoreV1().Pods(pod.Namespace).Get(context.Background(), pod.Name, metav1.GetOptions{})
	if err != nil {
		return hiperrors.ExitCodeUnknown
	}
	return exitCodeFromContainerStatuses(myPod.Status.ContainerStatuses)
}

// exitCodeFromContainerStatuses extracts the exit code from container statuses.
// Returns ExitCodeUnknown if no terminated container is found.
func exitCodeFromContainerStatuses(statuses []corev1.ContainerStatus) int32 {
	for _, cs := range statuses {
		if cs.State.Terminated != nil {
			return cs.State.Terminated.ExitCode
		}
	}
	return hiperrors.ExitCodeUnknown
}

// parseExitCodeFromError attempts to extract an exit code from an error message.
// The Kubernetes remotecommand executor produces messages like
// "command terminated with exit code N" which get wrapped by operatorkclient.
// Returns ExitCodeUnknown if the exit code cannot be determined.
func parseExitCodeFromError(err error) int {
	if err == nil {
		return hiperrors.ExitCodeUnknown
	}
	msg := err.Error()
	const prefix = "exit code "
	idx := strings.LastIndex(msg, prefix)
	if idx < 0 {
		return hiperrors.ExitCodeUnknown
	}
	codeStr := strings.TrimSpace(msg[idx+len(prefix):])
	// Take only digits
	end := 0
	for end < len(codeStr) && codeStr[end] >= '0' && codeStr[end] <= '9' {
		end++
	}
	if end == 0 {
		return hiperrors.ExitCodeUnknown
	}
	code := 0
	for _, c := range codeStr[:end] {
		code = code*10 + int(c-'0')
	}
	return code
}

// exitCodeMarkerWriter wraps an io.Writer and intercepts the exit code marker
// line emitted by the pod script in copy-from mode. The marker line is consumed
// (not forwarded to the underlying writer) and the exit code is stored.
type exitCodeMarkerWriter struct {
	inner      io.Writer
	found      bool
	exitCode   int
	cancelFunc context.CancelFunc
}

func newExitCodeMarkerWriter(inner io.Writer, cancel context.CancelFunc) *exitCodeMarkerWriter {
	return &exitCodeMarkerWriter{inner: inner, cancelFunc: cancel}
}

func (w *exitCodeMarkerWriter) Write(p []byte) (int, error) {
	if w.found {
		return w.inner.Write(p)
	}
	s := string(p)
	prefix := hipconsts.CopyFromExitCodeMarkerPrefix
	suffix := hipconsts.CopyFromExitCodeMarkerSuffix
	if strings.Contains(s, prefix) {
		// Extract exit code between prefix and suffix
		after, _ := strings.CutPrefix(s[strings.Index(s, prefix):], prefix)
		if codeStr, ok := strings.CutSuffix(after, suffix+"\n"); !ok {
			codeStr, _ = strings.CutSuffix(after, suffix)
			after = codeStr
		} else {
			after = codeStr
		}
		code, err := strconv.Atoi(strings.TrimSpace(after))
		if err == nil {
			w.found = true
			w.exitCode = code
			w.cancelFunc()
		}
		return len(p), nil
	}
	return w.inner.Write(p)
}

func (w *exitCodeMarkerWriter) Found() bool {
	return w.found
}

func (w *exitCodeMarkerWriter) ExitCode() int {
	return w.exitCode
}

// SignalCopyDone creates the sentinel file in the pod to let it know
// that copy-from is complete and it can exit.
func (m *Manager) SignalCopyDone(pod *corev1.Pod) {
	logz.HostPod().Debug().Msg("Signaling copy-done")
	_, _, err := m.client().RunCommandInPod(
		fmt.Sprintf("touch %s", hipconsts.CopyFromDoneFile),
		hipconsts.HelmInPodNamespace, pod.Name, pod.Namespace, nil)
	if err != nil {
		logz.Host().Debug().Msgf("Failed to signal copy-done (pod may have already exited): %v", err)
	}
}
