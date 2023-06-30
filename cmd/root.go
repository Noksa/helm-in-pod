package cmd

import (
	"fmt"
	"github.com/Noksa/operator-home/pkg/operatorkclient"
	"github.com/noksa/helm-in-pod/internal"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go.uber.org/multierr"
	"helm.sh/helm/v3/pkg/cli"
	"strings"
	"time"
)

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "in-pod",
		Short: "Run any helm command in pod",
	}
	opts := internal.HelmInPodFlags{}
	rootCmd.Flags().StringToStringVarP(&opts.Env, "env", "e", map[string]string{}, "Environment variables to be set before running a command")
	rootCmd.Flags().BoolVar(&opts.CopyRepo, "copy-repo", true, "Copy existing helm repositories to helm pod")
	rootCmd.Flags().StringSliceVar(&opts.UpdateRepo, "update-repo", []string{}, "A lit of helm repository aliases which should be updated before running a command. Applicable only if --copy-repo set to true")
	rootCmd.Flags().StringVarP(&opts.Image, "image", "i", "docker.io/noksa/kubectl-helm:v1.25.8-v3.10.3", "An image which will be used. Must contain helm")
	rootCmd.Flags().StringVarP(&opts.Files, "file", "f", "", "A map of files which should be copied from host to container. Can be specified multiple times. Example: -f /path_on_host/values.yaml:/path_in_container/values.yaml")
	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		startTime := time.Now()
		log.Warn("Executing helm-in-pod")
		defer func() {
			log.Warnf("Executing helm-in-pod took %v", time.Since(startTime))
		}()
		defer internal.Namespace.DeleteClusterRoleBinding()
		defer internal.Pod.DeleteAllPods()
		if opts.Files != "" {
			opts.FilesAsMap = map[string]string{}
			entries := strings.Split(opts.Files, ",")
			for _, v := range entries {
				splitted := strings.Split(v, ":")
				opts.FilesAsMap[splitted[0]] = splitted[1]
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
			err = internal.Pod.CopyFileToPod(pod, settings.RepositoryConfig, "/root/.config/helm/repositories.yaml")
			if err != nil {
				return err
			}
			for _, repo := range opts.UpdateRepo {
				log.Infof("%v Fetching updates from %v helm repository", internal.LogPod(), repo)
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
		cmdToUse := fmt.Sprintf("%v %v", "helm", strings.Join(args, " "))
		log.Infof("%v Running '%v' command", internal.LogPod(), cmdToUse)
		stdout, err := operatorkclient.RunCommandInPod(cmdToUse, internal.HelmInPodNamespace, pod.Name, pod.Namespace, nil)
		if err != nil {
			return multierr.Append(err, fmt.Errorf(stdout))
		}
		log.Info()
		log.Info("Output:")
		fmt.Printf(stdout)
		return nil
	}
	return rootCmd
}

func ExecuteRoot(args []string) (err error) {
	rootCmd := newRootCmd()
	rootCmd.SilenceUsage = true
	rootCmd.SetArgs(args)
	return internal.RunCommand(rootCmd)
}
