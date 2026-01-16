package main

import (
	"github.com/noksa/helm-in-pod/cmd"
	log "github.com/sirupsen/logrus"
	"os"
)

func main() {
	log.SetOutput(os.Stderr)
	err := cmd.ExecuteRoot()
	if err != nil {
		log.Fatalf("%s", err.Error())
		os.Exit(100)
	}
}
