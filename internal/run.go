package internal

import (
	"context"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
	"time"
)

func RunCommand(cmd *cobra.Command) (context.Context, error) {
	flags := cmd.PersistentFlags()
	flags.ParseErrorsWhitelist.UnknownFlags = true
	_ = flags.Parse(os.Args[1:])
	dur, err := cmd.PersistentFlags().GetDuration("timeout")
	if err != nil {
		return ctx, err
	}
	if dur <= 0 {
		log.Info("Sets default timeout to 2h")
		dur = time.Hour * 2
	}
	ctx, cancel = context.WithTimeout(context.Background(), dur)
	err = cmd.ExecuteContext(ctx)
	cancel()
	return ctx, err
}
