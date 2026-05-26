package main

import (
	"os"

	_ "github.com/lib/pq" // postgres driver

	retrowinserver "github.com/mandacode-labs/retrowin-go/internal/cmd/retrowin-server"
)

var version = "dev"

func main() {
	cmd := retrowinserver.NewCmd()
	cmd.Version = version
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
