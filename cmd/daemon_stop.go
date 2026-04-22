package cmd

import (
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/noksa/helm-in-pod/internal"
	"github.com/noksa/helm-in-pod/internal/logz"
)

func newDaemonStopCmd() *cobra.Command {
	var name string
	stopCmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop and delete a daemon pod",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			name, err = getDaemonName(name)
			if err != nil {
				return err
			}
			_, err = internal.Pod().GetDaemonPod(name)
			if err != nil {
				if k8serrors.IsNotFound(err) {
					logz.Host().Info().Msgf("Daemon %s doesn't exist", color.CyanString(name))
					return nil
				}
				return err
			}
			return internal.Pod().DeleteDaemonPod(name)
		},
	}
	stopCmd.Flags().StringVar(&name, "name", "", "Daemon name (required)")
	return stopCmd
}
