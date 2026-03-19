package cmd

import (
	"github.com/fatih/color"
	"github.com/noksa/helm-in-pod/internal"
	"github.com/noksa/helm-in-pod/internal/logz"
	"github.com/spf13/cobra"
)

func newDaemonShellCmd() *cobra.Command {
	var name string
	var shell string
	shellCmd := &cobra.Command{
		Use:   "shell",
		Short: "Open an interactive shell in a daemon pod",
		Long: `Open an interactive shell session in a daemon pod with full Helm context.
All Helm repositories and configurations are already set up and ready to use.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			name, err = getDaemonName(name)
			if err != nil {
				return err
			}

			logz.Host().Debug().Msgf("Looking for %s daemon", color.CyanString(name))
			pod, err := internal.Pod().GetDaemonPod(name)
			if err != nil {
				return err
			}
			logz.Host().Info().Msgf("Opening interactive shell in %s daemon", color.CyanString(pod.Name))
			logz.Host().Info().Msg("Type 'exit' or press Ctrl+D to close the shell")

			return internal.Pod().OpenInteractiveShell(cmd.Context(), pod, shell)
		},
	}
	shellCmd.Flags().StringVar(&name, "name", "", "Daemon name (required)")
	shellCmd.Flags().StringVar(&shell, "shell", "sh", "Shell to use (sh, bash, zsh, etc.)")
	return shellCmd
}
