package internal

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/noksa/helm-in-pod/internal/logz"
)

func RunCommand(cmd *cobra.Command) error {
	flags := cmd.PersistentFlags()
	flags.ParseErrorsAllowlist.UnknownFlags = true
	_ = flags.Parse(os.Args[1:])
	dur, err := cmd.PersistentFlags().GetDuration("timeout")
	if err != nil {
		return err
	}
	if dur <= 0 {
		logz.Host().Debug().Msg("Sets default timeout to 2h")
		dur = time.Hour * 2
	}
	// Create a context that is canceled on SIGINT/SIGTERM so all downstream
	// goroutines (file copy, repo update, command execution) stop immediately
	// when the user presses Ctrl+C.
	sigCtx, sigStop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer sigStop()

	ctx, cancel := context.WithTimeout(sigCtx, dur)
	defer cancel()
	return cmd.ExecuteContext(ctx)
}
