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
		Short: "Removes remaining pods/garbage/etc in a k8s cluster",
	}
	opts := cmdoptions.PurgeOptions{}
	purgeCmd.Flags().BoolVar(&opts.All, "all", false, "Removes all things which were created in a k8s cluster")
	purgeCmd.RunE = func(cmd *cobra.Command, args []string) error {
		return errors.Join(
			internal.Namespace().DeleteClusterRoleBinding(),
			internal.Pod().DeleteHelmPods(cmdoptions.ExecOptions{}, opts),
		)
	}
	return purgeCmd
}
