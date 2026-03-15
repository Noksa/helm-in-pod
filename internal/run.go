package internal

import (
	"context"
	"os"
	"time"

	"github.com/noksa/helm-in-pod/internal/logz"
	"github.com/spf13/cobra"
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
	ctx, cancel := context.WithTimeout(context.Background(), dur)
	defer cancel()
	return cmd.ExecuteContext(ctx)
}
