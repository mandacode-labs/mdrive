// Package gc implements the gc command
package gc

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/mandacode-labs/mdrive/internal/config"
	"github.com/mandacode-labs/mdrive/internal/logging"
)

// NewCmd creates a new cobra command for the gc command
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gc",
		Short: "Garbage collection commands",
	}

	cmd.AddCommand(newRunCmd())

	return cmd
}

// newRunCmd creates the gc run subcommand
func newRunCmd() *cobra.Command {
	var cfgFile string
	var pendingExpiry string

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run garbage collection (expired pending + orphan cleanup)",
		RunE: func(cmd *cobra.Command, args []string) error {
			var cfg *config.Config
			var err error

			if cfgFile != "" {
				cfg, err = config.LoadFromPath(cfgFile)
			} else {
				cfg, err = config.Load("config.yaml")
			}
			if err != nil {
				return err
			}

			// Parse pending expiry duration
			expiry := time.Duration(0)
			if pendingExpiry != "" {
				expiry, err = time.ParseDuration(pendingExpiry)
				if err != nil {
					return fmt.Errorf("invalid pending-expiry duration: %w", err)
				}
			}

			// Create logger
			logger := logging.NewLogger(cfg.App.Env, cfg.App.LogLevel)

			return runGC(cfg, expiry, logger)
		},
	}

	cmd.Flags().StringVarP(&cfgFile, "config", "c", "", "config file path")
	cmd.Flags().StringVar(&pendingExpiry, "pending-expiry", "", "pending object expiry duration (e.g. 24h, 48h)")

	return cmd
}
