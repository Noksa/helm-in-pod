package cmd

import (
	"github.com/spf13/cobra"
)

func newDaemonCmd() *cobra.Command {
	daemonCmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage persistent pods for repeated command execution",
	}
	daemonCmd.AddCommand(
		newDaemonStartCmd(),
		newDaemonStopCmd(),
		newDaemonExecCmd(),
		newDaemonShellCmd(),
		newDaemonStatusCmd(),
		newDaemonListCmd())
	return daemonCmd
}
