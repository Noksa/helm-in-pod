package hippod

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Noksa/operator-home/pkg/operatorkclient"
	"github.com/fatih/color"
	"github.com/noksa/helm-in-pod/internal/cmdoptions"
	"github.com/noksa/helm-in-pod/internal/hipretry"
	"github.com/noksa/helm-in-pod/internal/logz"
	log "github.com/sirupsen/logrus"
	"go.uber.org/multierr"
	"helm.sh/helm/v4/pkg/cli"
	corev1 "k8s.io/api/core/v1"
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
		log.Debugf("%v Determining user home directory", logz.LogPod())
		var stderr string
		var err error
		stdout, stderr, err = operatorkclient.RunCommandInPod(
			`echo "${HOME}:::$(whoami):::$(id)"`,
			Namespace, pod.Name, pod.Namespace, nil)
		if err != nil {
			return multierr.Append(fmt.Errorf("%s", stderr), err)
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

	log.Debugf("%v (%v) home directory: %v", logz.LogPod(), color.GreenString(whoami), color.MagentaString(homeDirectory))
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
		log.Debugf("%v Creating %v/.config/helm directory", logz.LogPod(), homeDirectory)
		_, stderr, err := operatorkclient.RunCommandInPod(
			`set +e; mkdir -p "${HOME}/.config/helm" &>/dev/null`,
			Namespace, pod.Name, pod.Namespace, nil)
		if err != nil {
			return multierr.Append(fmt.Errorf("%s", stderr), err)
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

func (m *Manager) updateHelmRepositories(pod *corev1.Pod, opts cmdoptions.ExecOptions, isHelm4 bool) error {
	if len(opts.UpdateRepo) == 0 {
		return hipretry.Retry(opts.UpdateRepoAttempts, func() error {
			log.Infof("%v Fetching updates from %v helm repositories", logz.LogPod(), color.GreenString("all"))
			cmdToUse := "helm repo update"
			if !isHelm4 {
				cmdToUse = fmt.Sprintf("%v --fail-on-repo-update-fail", cmdToUse)
			}
			stdout, stderr, err := operatorkclient.RunCommandInPod(cmdToUse,
				Namespace, pod.Name, pod.Namespace, nil)
			if err != nil {
				return multierr.Append(err, fmt.Errorf("%v\n%v", stdout, stderr))
			}
			log.Debugf("%v Helm repository updates have been fetched", logz.LogPod())
			return nil
		})
	}

	var mErr error
	for _, repo := range opts.UpdateRepo {
		err := hipretry.Retry(opts.UpdateRepoAttempts, func() error {
			log.Infof("%v Fetching updates from %v helm repository", logz.LogPod(), color.CyanString(repo))
			cmdToUse := fmt.Sprintf("helm repo update %v", repo)
			if !isHelm4 {
				cmdToUse = fmt.Sprintf("%v --fail-on-repo-update-fail", cmdToUse)
			}
			stdout, stderr, err := operatorkclient.RunCommandInPod(cmdToUse,
				Namespace, pod.Name, pod.Namespace, nil)
			if err != nil {
				return multierr.Append(err, fmt.Errorf("%v\n%v", stdout, stderr))
			}
			log.Debugf("%v %v helm repository updates have been fetched", logz.LogPod(), color.CyanString(repo))
			return nil
		})
		if err != nil {
			mErr = multierr.Append(mErr, err)
		}
	}
	return mErr
}

func (m *Manager) CopyUserFiles(pod *corev1.Pod, opts cmdoptions.ExecOptions, expandPath func(string) (string, error)) error {
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
	scriptPath := fmt.Sprintf("%v/wrapped-script.sh", homeDirectory)

	tempScriptFile, err := os.CreateTemp("", "helm-in-pod")
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
	_, err = tempScriptFile.WriteString(command)
	if err != nil {
		return err
	}

	since := time.Now()
	err = m.CopyFileToPod(pod, tempScriptFile.Name(), scriptPath, opts.CopyAttempts)
	if err != nil {
		return err
	}

	log.Infof("%v Running '%v' command", logz.LogPod(), color.YellowString(command))

	b := &bytes.Buffer{}
	multiWriter := io.MultiWriter(os.Stdout, b)

	go func() {
		<-ctx.Done()
		log.Warnf("%v Timed out!", logz.LogHost())
		for {
			_, _, err := operatorkclient.RunCommandInPod("kill -term 1",
				"helm-in-pod", pod.Name, pod.Namespace, nil)
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
				return
			}
			err = m.StreamLogsFromPod(context.Background(), pod, multiWriter, since)
			since = time.Now()
			if err == nil {
				return
			}
			log.Infof("got an error from streaming pod logs: %v", err)
			time.Sleep(time.Millisecond * 25)
		}
	})
	wg.Wait()

	return m.waitForPodCompletion(ctx, pod)
}

func (m *Manager) waitForPodCompletion(ctx context.Context, pod *corev1.Pod) error {
	log.Debugf("%v Waiting 60s until pod phase is changed to failed/succeeded", logz.LogHost())

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

	log.Debugf("%v Pod got phase: %v", logz.LogHost(), color.CyanString("%v", phase))

	if phase == corev1.PodFailed {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("pod failed")
	}
	if phase == corev1.PodSucceeded {
		return nil
	}
	return fmt.Errorf("unexpected pod phase: %v", phase)
}
