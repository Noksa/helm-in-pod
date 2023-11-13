package cmd

import (
	"context"
	"errors"
	"fmt"
	"github.com/Noksa/operator-home/pkg/operatorkclient"
	"github.com/fatih/color"
	"github.com/noksa/helm-in-pod/internal"
	"github.com/noksa/helm-in-pod/internal/cmdoptions"
	"github.com/noksa/helm-in-pod/internal/logz"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go.uber.org/multierr"
	"helm.sh/helm/v3/pkg/cli"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"
)

func newExecCmd() *cobra.Command {
	execCmd := &cobra.Command{
		Use:     "exec [flags] -- <helm_command_to_run>",
		Aliases: []string{"run"},
		Short:   "Executes commands in helm pod",
	}
	opts := cmdoptions.ExecOptions{}
	execCmd.Flags().Int64Var(&opts.RunAsUser, "run-as-user", -1, "Run as user ID to be set in security context. Omitted if not specified and default from an image is used")
	execCmd.Flags().Int64Var(&opts.RunAsGroup, "run-as-group", -1, "Run as group ID to be set in security context. Omitted if not specified and default from an image is used")
	execCmd.Flags().StringToStringVar(&opts.Labels, "labels", map[string]string{}, "Additional labels to be set on a pod")
	execCmd.Flags().DurationVar(&opts.Timeout, "timeout", time.Hour*1, "After timeout a helm-pod will be terminated even if a command is still running")
	execCmd.Flags().StringVar(&opts.Cpu, "cpu", "1100m", "Pod's cpu request/limit")
	execCmd.Flags().StringVar(&opts.Memory, "memory", "500Mi", "Pod's memory request/limit")
	execCmd.Flags().StringToStringVarP(&opts.Env, "env", "e", map[string]string{}, "Environment variables to be set in helm's pod before running a command")
	execCmd.Flags().StringSliceVarP(&opts.SubstEnv, "subst-env", "s", []string{}, "Environment variables to be substituted in helm's pod (WITHOUT values). Values will be substituted from exported env on host")
	execCmd.Flags().StringVar(&opts.ImagePullSecret, "image-pull-secret", "", "Specify an image pull secret which should be used to pull --image from private repository")
	execCmd.Flags().BoolVar(&opts.CopyRepo, "copy-repo", true, "Copy existing helm repositories to helm pod")
	execCmd.Flags().StringVar(&opts.PullPolicy, "pull-policy", "IfNotPresent", "Image pull policy to use in helm pod")
	execCmd.Flags().StringSliceVar(&opts.UpdateRepo, "update-repo", []string{}, "A list of helm repository aliases which should be updated before running a command. Applicable only if --copy-repo set to true")
	execCmd.Flags().StringVarP(&opts.Image, "image", "i", "docker.io/noksa/kubectl-helm:v1.25.8-v3.10.3", "An image which will be used. Must contain helm")
	execCmd.Flags().StringSliceVarP(&opts.Files, "copy", "c", []string{}, "A map of files/directories which should be copied from host to container. Can be specified multiple times. Example: -c /path_on_host/values.yaml:/path_in_container/values.yaml")
	execCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("specify command to run. Run `helm inpod exec --help` to check available options")
		}
		if opts.Timeout < time.Second*1 {
			return fmt.Errorf("timeout can't be less 1s")
		}
		var mErr error
		defer multierr.AppendInvoke(&mErr, multierr.Invoke(func() error {
			deferErr := internal.Pod.DeleteHelmPods(opts, cmdoptions.PurgeOptions{All: false})
			return deferErr
		}))
		if len(opts.Files) > 0 {
			opts.FilesAsMap = map[string]string{}
			for _, val := range opts.Files {
				entries := strings.Split(val, ",")
				for _, v := range entries {
					splitted := strings.Split(v, ":")
					opts.FilesAsMap[splitted[0]] = splitted[1]
				}
			}
		}
		err := internal.Namespace.PrepareNs()
		if err != nil {
			return err
		}
		pod, err := internal.Pod.CreateHelmPod(opts)
		if err != nil {
			return err
		}
		if opts.CopyRepo {
			settings := cli.New()
			_, statErr := os.Stat(settings.RepositoryConfig)
			if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
				return statErr
			}
			if statErr == nil {
				// determine user home directory
				attempts := 3
				var aErr, rErr error
				var stdout, stderr string
				for i := 0; i < attempts; i++ {
					log.Debugf("%v Determining user home directory", logz.LogPod())
					stdout, stderr, rErr = operatorkclient.RunCommandInPod(`set +e; mkdir -p "${HOME}/.config/helm" &>/dev/null; echo "${HOME}:::$(whoami):::$(id)"`, internal.HelmInPodNamespace, pod.Name, pod.Namespace, nil)
					if rErr == nil {
						aErr = nil
						break
					}
					aErr = multierr.Append(aErr, fmt.Errorf(stderr))
					aErr = multierr.Append(aErr, rErr)
					time.Sleep(time.Second)
				}
				if aErr != nil {
					return aErr
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
				userInfo := fmt.Sprintf("id: %v, whoami: %v", color.GreenString(id), color.YellowString(whoami))
				if homeDirectory == "" {
					return fmt.Errorf("user (%v) in the image doesn't have home directory", userInfo)
				}
				log.Debugf("%v (%v) home directory: %v", logz.LogPod(), color.GreenString(whoami), color.MagentaString(homeDirectory))
				err = internal.Pod.CopyFileToPod(pod, settings.RepositoryConfig, fmt.Sprintf("%v/.config/helm/repositories.yaml", homeDirectory))
				if err != nil {
					return err
				}
			}
			for _, repo := range opts.UpdateRepo {
				log.Infof("%v Fetching updates from %v helm repository", logz.LogPod(), color.CyanString(repo))
				stdout, stderr, err := operatorkclient.RunCommandInPod(fmt.Sprintf("helm repo update %v --fail-on-repo-update-fail", repo), internal.HelmInPodNamespace, pod.Name, pod.Namespace, nil)
				if err != nil {
					return multierr.Append(err, fmt.Errorf("%v\n%v", stdout, stderr))
				}
			}
		}
		for k, v := range opts.FilesAsMap {
			path, err := expand(k)
			if err != nil {
				return err
			}
			err = internal.Pod.CopyFileToPod(pod, path, v)
			if err != nil {
				return err
			}
		}
		cmdToUse := strings.Join(args, " ")
		log.Infof("%v Running '%v' command", logz.LogPod(), color.YellowString(cmdToUse))

		// do not use stdout and stderr vars, we are going to stream everything to stdout
		_, _, err = operatorkclient.RunCommandInPodWithOptions(operatorkclient.RunCommandInPodOptions{
			Context:       context.Background(),
			Timeout:       opts.Timeout,
			Command:       cmdToUse,
			PodName:       pod.Name,
			PodNamespace:  pod.Namespace,
			ContainerName: internal.HelmInPodNamespace,
			Stdin:         nil,
			Stderr:        os.Stdout,
			Stdout:        os.Stdout,
		})
		return err
	}
	return execCmd
}

func expand(path string) (string, error) {
	if len(path) == 0 || path[0] != '~' {
		return path, nil
	}

	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	return filepath.Join(usr.HomeDir, path[1:]), nil
}
