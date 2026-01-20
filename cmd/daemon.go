package cmd

import (
	"github.com/spf13/cobra"
)

func newDaemonCmd() *cobra.Command {
	daemonCmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage long-running pods in the cluster",
	}
	daemonCmd.AddCommand(
		newDaemonStartCmd(),
		newDaemonStopCmd(),
		newDaemonExecCmd(),
		newDaemonShellCmd())
	return daemonCmd
}
