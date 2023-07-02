package cmd

import (
	"github.com/noksa/helm-in-pod/internal"
	"github.com/noksa/helm-in-pod/internal/cmdoptions"
	"github.com/spf13/cobra"
	"go.uber.org/multierr"
)

func newPurgeCmd() *cobra.Command {
	purgeCmd := &cobra.Command{
		Use:   "purge",
		Short: "Removes remaining pods/garbage/etc in a k8s cluster",
	}
	opts := cmdoptions.PurgeOptions{}
	purgeCmd.Flags().BoolVar(&opts.All, "all", false, "Removes all things which were created in a k8s cluster")
	purgeCmd.RunE = func(cmd *cobra.Command, args []string) error {
		var mErr error
		mErr = multierr.Append(mErr, internal.Namespace.DeleteClusterRoleBinding())
		mErr = multierr.Append(mErr, internal.Pod.DeleteHelmPods(opts.All))
		return mErr
	}
	return purgeCmd
}
