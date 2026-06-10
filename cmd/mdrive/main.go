package main

import (
	"os"

	_ "github.com/lib/pq" // postgres driver

	mdrivecmd "github.com/mandacode-labs/mdrive/internal/cmd/mdrive"
)

var version = "dev"

func main() {
	cmd := mdrivecmd.NewCmd()
	cmd.Version = version
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
