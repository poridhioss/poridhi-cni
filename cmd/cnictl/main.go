package main

import (
	"os"

	"github.com/poridhioss/poridhi-cni/cmd/cnictl/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}


