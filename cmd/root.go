package cmd

import (
	"github.com/fatih/color"
	"github.com/noksa/helm-in-pod/internal"
	"github.com/noksa/helm-in-pod/internal/helpers"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
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
	debug := false
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug logs")
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if debug {
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

func ExecuteRoot(args []string) (err error) {
	rootCmd := newRootCmd()
	rootCmd.SilenceUsage = true
	rootCmd.SetArgs(args)
	return internal.RunCommand(rootCmd)
}
