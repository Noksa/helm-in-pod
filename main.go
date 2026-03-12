package main

import (
	"errors"
	"os"

	"github.com/noksa/helm-in-pod/cmd"
	"github.com/noksa/helm-in-pod/internal/hiperrors"
	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetOutput(os.Stderr)
	err := cmd.ExecuteRoot()
	if err != nil {
		var exitErr *hiperrors.ExitCodeError
		if errors.As(err, &exitErr) {
			os.Exit(int(exitErr.Code))
		}
		log.Fatalf("%s", err.Error())
	}
}
