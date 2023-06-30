package cmd

import (
	"fmt"
	"github.com/Noksa/operator-home/pkg/operatorkclient"
	"github.com/noksa/helm-in-pod/internal"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go.uber.org/multierr"
	"os"
	"strings"
)

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "in-pod",
		Short: "Run any helm command in pod",
	}
	opts := internal.HelmInPodFlags{}
	rootCmd.Flags().StringVar(&opts.Image, "image", "docker.io/noksa/kubectl-helm:v1.25.8-v3.10.3", "An image which will be used. Must contain helm")
	rootCmd.Flags().StringVarP(&opts.Files, "file", "f", "", "A map of files which should be copied from host to container. Can be specified multiple times. Example: -f /path_on_host/values.yaml:/path_in_container/values.yaml")
	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		log.SetOutput(os.Stderr)
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
		for k, v := range opts.FilesAsMap {
			err = internal.Pod.CopyFileToPod(pod, k, v)
			if err != nil {
				return err
			}
		}
		cmdToUse := fmt.Sprintf("%v %v", "helm", strings.Join(args, " "))
		log.Infof("Running '%v' command in '%v' pod", cmdToUse, pod.Name)
		stdout, err := operatorkclient.RunCommandInPod(cmdToUse, internal.HelmInPodNamespace, pod.Name, pod.Namespace, nil)
		if err != nil {
			return multierr.Append(err, fmt.Errorf(stdout))
		}
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
