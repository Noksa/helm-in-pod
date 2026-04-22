package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/noksa/helm-in-pod/cmd"
	"github.com/noksa/helm-in-pod/internal/hiperrors"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs
	cw := zerolog.ConsoleWriter{
		Out:           os.Stderr,
		TimeFormat:    "15:04:05.000",
		PartsOrder:    []string{"time", "level", "message", "source"},
		FieldsExclude: []string{"source"},
		FormatPartValueByName: func(i any, name string) string {
			if name != "source" {
				return fmt.Sprintf("%s", i)
			}
			s := fmt.Sprintf("%s", i)
			switch s {
			case "host":
				return fmt.Sprintf("[%s]", color.CyanString("host"))
			case "pod":
				return fmt.Sprintf("[%s]", color.MagentaString("pod"))
			case "host+pod":
				return fmt.Sprintf("[%s+%s]", color.CyanString("host"), color.MagentaString("pod"))
			default:
				return fmt.Sprintf("[%s]", s)
			}
		},
	}
	log.Logger = zerolog.New(cw).With().Timestamp().Logger()
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
