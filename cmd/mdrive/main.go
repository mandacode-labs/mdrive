// Package main is the mdrive entry point.
package main

import (
	"fmt"
	"os"

	"github.com/mandacode-labs/mdrive/internal/cmd/serve"
	"github.com/mandacode-labs/mdrive/internal/config"
)

const version = "dev"

func main() {
	cfgPath := os.Getenv("MDRIVE_CONFIG")
	if cfgPath == "" {
		cfgPath = "config.yaml"
	}
	cfg, err := config.LoadFromPath(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}
	if err := serve.Run(cfg, version); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}
