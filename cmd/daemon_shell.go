package cmd

import (
	"github.com/fatih/color"
	"github.com/noksa/helm-in-pod/internal"
	"github.com/noksa/helm-in-pod/internal/logz"
	log "github.com/sirupsen/logrus"
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

			log.Debugf("%s Looking for %s daemon", logz.LogHost(), color.CyanString(name))
			pod, err := internal.Pod.GetDaemonPod(name)
			if err != nil {
				return err
			}
			log.Infof("%s Opening interactive shell in %s daemon", logz.LogHost(), color.CyanString(pod.Name))
			log.Infof("Type 'exit' or press Ctrl+D to close the shell")

			return internal.Pod.OpenInteractiveShell(cmd.Context(), pod, shell)
		},
	}
	shellCmd.Flags().StringVar(&name, "name", "", "Daemon name (required)")
	shellCmd.Flags().StringVar(&shell, "shell", "sh", "Shell to use (sh, bash, zsh, etc.)")
	return shellCmd
}
