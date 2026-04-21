package cmd

import (
	"errors"

	"github.com/noksa/helm-in-pod/internal"
	"github.com/noksa/helm-in-pod/internal/cmdoptions"
	"github.com/spf13/cobra"
)

func newPurgeCmd() *cobra.Command {
	purgeCmd := &cobra.Command{
		Use:   "purge",
		Short: "Remove leftover pods and cluster resources created by the plugin",
	}
	opts := cmdoptions.PurgeOptions{}
	purgeCmd.Flags().BoolVar(&opts.All, "all", false, "Remove all pods in the helm-in-pod namespace (regardless of host), associated PDBs, and the ClusterRoleBinding")
	purgeCmd.RunE = func(cmd *cobra.Command, args []string) error {
		return errors.Join(
			internal.Namespace().DeleteClusterRoleBinding(),
			internal.Pod().DeleteHelmPods(cmdoptions.ExecOptions{}, opts),
		)
	}
	return purgeCmd
}
