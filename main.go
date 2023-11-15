package main

import (
	"github.com/noksa/helm-in-pod/cmd"
	log "github.com/sirupsen/logrus"
	"os"
)

func main() {
	log.SetOutput(os.Stderr)
	ctx, err := cmd.ExecuteRoot()
	if err != nil {
		if ctx.Err() != nil {
			log.Println(ctx.Err())
			os.Exit(2)
		} else {
			log.Println(err)
			os.Exit(1)
		}
	}
}
