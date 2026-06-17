// Package gc implements the `mdrive gc` subcommand.
package gc

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/mandacode-labs/mdrive/internal/app"
	"github.com/mandacode-labs/mdrive/internal/app/gc"
	"github.com/mandacode-labs/mdrive/internal/config"
)

// NewCmd creates the `gc` subcommand.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gc",
		Short: "Run the GC (garbage collection) job",
	}
	cmd.AddCommand(newRunCmd())
	return cmd
}

func newRunCmd() *cobra.Command {
	var (
		configPath string
		tick       time.Duration
	)
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the GC job (one-shot by default, periodic with --tick)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.LoadFromPath(configPath)
			if err != nil {
				return err
			}
			a, err := app.New(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			defer func() { _ = a.Close() }()
			return gc.Run(cmd.Context(), a, gc.Config{Tick: tick})
		},
	}
	cmd.Flags().StringVarP(&configPath, "config", "c", "config.yaml", "path to config file")
	cmd.Flags().DurationVar(&tick, "tick", 0, "if set (>0), run periodically at this interval instead of one-shot")
	return cmd
}
