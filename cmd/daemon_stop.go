package cmd

import (
	"fmt"

	"github.com/noksa/helm-in-pod/internal"
	"github.com/spf13/cobra"
)

func newDaemonStopCmd() *cobra.Command {
	var name string
	stopCmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop a daemon pod",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return fmt.Errorf("--name is required")
			}
			return internal.Pod.DeleteDaemonPod(name)
		},
	}
	stopCmd.Flags().StringVar(&name, "name", "", "Daemon name (required)")
	return stopCmd
}
