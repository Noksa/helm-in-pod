package main

import (
	"github.com/noksa/helm-in-pod/cmd"
	"os"
)

func main() {
	err := cmd.ExecuteRoot(os.Args[1:])
	if err != nil {
		os.Exit(1)
	}
}
