package main

import (
	"github.com/noksa/helm-in-pod/cmd"
	log "github.com/sirupsen/logrus"
	"os"
)

func main() {
	log.SetOutput(os.Stderr)
	err := cmd.ExecuteRoot(os.Args[1:])
	if err != nil {
		os.Exit(1)
	}
}
