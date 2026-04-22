package cmd

import (
	"fmt"
	"time"

	"github.com/fatih/color"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/noksa/helm-in-pod/internal"
	"github.com/noksa/helm-in-pod/internal/helpers"
	"github.com/noksa/helm-in-pod/internal/logz"
)

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "in-pod",
		Short: "Run any command inside a Kubernetes cluster",
	}
	rootCmd.AddCommand(
		newExecCmd(),
		newPurgeCmd(),
		newDaemonCmd())

	startTime := time.Now()
	var debug bool
	rootCmd.PersistentFlags().BoolVar(&debug, "verbose-logs", false, "Enable debug logs")
	rootCmd.PersistentFlags().Duration("timeout", time.Second*0, "Gracefully terminate the command after this duration (default: 2h at runtime). For exec and daemon start, 10 extra minutes are added internally for pod operations")
	_ = viper.BindPFlag("timeout", rootCmd.PersistentFlags().Lookup("timeout"))
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if debug {
			logz.Host().Info().Msg("Setting log level to debug")
			zerolog.SetGlobalLevel(zerolog.DebugLevel)
		}
		if !helpers.IsCompletionCmd(cmd) {
			logz.Host().Info().Msgf("Running %v command", color.CyanString(cmd.Name()))
		}
		if err := internal.InitManagers(); err != nil {
			return fmt.Errorf("could not initialize Kubernetes client: %w", err)
		}
		return nil
	}
	rootCmd.PersistentPostRunE = func(cmd *cobra.Command, args []string) error {
		if !helpers.IsCompletionCmd(cmd) {
			logz.Host().Info().Msgf("%v command took %v", color.CyanString(cmd.Name()), color.GreenString("%v", time.Since(startTime).Round(time.Millisecond)))
		}
		return nil
	}
	return rootCmd
}

func ExecuteRoot() error {
	rootCmd := newRootCmd()
	rootCmd.SilenceUsage = true
	return internal.RunCommand(rootCmd)
}
