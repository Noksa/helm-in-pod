package main

import (
	"errors"
	"os"

	"github.com/noksa/helm-in-pod/cmd"
	"github.com/noksa/helm-in-pod/internal/hiperrors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	log.Logger = zerolog.New(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: "15:04:05.000",
	}).With().Timestamp().Logger()
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	err := cmd.ExecuteRoot()
	if err != nil {
		var exitErr *hiperrors.ExitCodeError
		if errors.As(err, &exitErr) {
			os.Exit(int(exitErr.Code))
		}
		log.Fatal().Msg(err.Error())
	}
}
