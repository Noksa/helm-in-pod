package cmd

import (
	"github.com/fatih/color"
	"github.com/noksa/helm-in-pod/internal"
	"github.com/noksa/helm-in-pod/internal/helpers"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"time"
)

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "in-pod",
		Short: "Run helm commands in a pod",
	}
	rootCmd.AddCommand(
		newExecCmd(),
		newPurgeCmd())

	startTime := time.Now()
	var debug bool
	rootCmd.PersistentFlags().BoolVar(&debug, "verbose-logs", false, "Enable debug logs")
	rootCmd.PersistentFlags().Duration("timeout", time.Second*0, "After timeout a command will be gracefully terminated even if it is still running. Default is 1h")
	_ = viper.BindPFlag("timeout", rootCmd.PersistentFlags().Lookup("timeout"))
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if debug {
			log.Info("Setting log level to debug")
			log.SetLevel(log.DebugLevel)
		}
		if !helpers.IsCompletionCmd(cmd) {
			log.Warnf("Running %v command", color.CyanString(cmd.Name()))
		}
		return nil
	}
	rootCmd.PersistentPostRunE = func(cmd *cobra.Command, args []string) error {
		if !helpers.IsCompletionCmd(cmd) {
			log.Warnf("%v command took %v", color.CyanString(cmd.Name()), color.GreenString("%v", time.Since(startTime).Round(time.Millisecond)))
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
