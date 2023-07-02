package helpers

import "github.com/spf13/cobra"

func IsCompletionCmd(cmd *cobra.Command) bool {
	if cmd.Name() == "completion" {
		return true
	}
	for parent := cmd.Parent(); parent != nil; parent = parent.Parent() {
		if parent.Name() == "completion" {
			return true
		}
	}
	return false
}
