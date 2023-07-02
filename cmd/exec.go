package cmd

import (
	"errors"
	"fmt"
	"github.com/Noksa/operator-home/pkg/operatorkclient"
	"github.com/fatih/color"
	"github.com/noksa/helm-in-pod/internal"
	"github.com/noksa/helm-in-pod/internal/cmdoptions"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go.uber.org/multierr"
	"helm.sh/helm/v3/pkg/cli"
	"os"
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
	execCmd.Flags().StringToStringVar(&opts.Labels, "labels", map[string]string{}, "Additional labels to be set on a pod")
	execCmd.Flags().DurationVar(&opts.Timeout, "timeout", time.Hour*1, "After timeout a helm-pod will be terminated even if a command is still running")
	execCmd.Flags().StringVar(&opts.Cpu, "cpu", "1100m", "Pod's cpu request/limit")
	execCmd.Flags().StringVar(&opts.Memory, "memory", "500Mi", "Pod's memory request/limit")
	execCmd.Flags().StringToStringVarP(&opts.Env, "env", "e", map[string]string{}, "Environment variables to be set in helm's pod before running a command")
	execCmd.Flags().StringSliceVarP(&opts.SubstEnv, "subst-env", "s", []string{}, "Environment variables to be substituted in helm's pod (WITHOUT values). Values will be substituted from exported env on host")

	execCmd.Flags().BoolVar(&opts.CopyRepo, "copy-repo", true, "Copy existing helm repositories to helm pod")
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
			deferErr := internal.Namespace.DeleteClusterRoleBinding()
			deferErr = multierr.Append(deferErr, internal.Pod.DeleteHelmPods(opts, cmdoptions.PurgeOptions{All: false}))
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
				err = internal.Pod.CopyFileToPod(pod, settings.RepositoryConfig, "/root/.config/helm/repositories.yaml")
				if err != nil {
					return err
				}
			}
			for _, repo := range opts.UpdateRepo {
				log.Infof("%v Fetching updates from %v helm repository", internal.LogPod(), color.CyanString(repo))
				stdout, err := operatorkclient.RunCommandInPod(fmt.Sprintf("helm repo update %v --fail-on-repo-update-fail", repo), internal.HelmInPodNamespace, pod.Name, pod.Namespace, nil)
				if err != nil {
					return multierr.Append(err, fmt.Errorf(stdout))
				}
			}
		}
		for k, v := range opts.FilesAsMap {
			err = internal.Pod.CopyFileToPod(pod, k, v)
			if err != nil {
				return err
			}
		}
		cmdToUse := strings.Join(args, " ")
		log.Infof("%v Running '%v' command", internal.LogPod(), cmdToUse)
		stdout, err := operatorkclient.RunCommandInPodWithTimeout(opts.Timeout, cmdToUse, internal.HelmInPodNamespace, pod.Name, pod.Namespace, nil)
		if err != nil {
			return multierr.Append(err, fmt.Errorf(stdout))
		}
		log.Info()
		log.Info(fmt.Sprintf("%v:\n", color.GreenString("Result")))
		fmt.Print(stdout)
		return nil
	}
	return execCmd
}
