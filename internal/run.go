package internal

import "github.com/spf13/cobra"

func RunCommand(cmd *cobra.Command) error {
	return cmd.ExecuteContext(ctx)
}
