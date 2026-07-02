// Package gc implements the `mdrive gc` subcommand.
package gc

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/mandacode-labs/mdrive/internal/app"
	gcjobs "github.com/mandacode-labs/mdrive/internal/app/gc"
	"github.com/mandacode-labs/mdrive/internal/config"
)

// addJob wires a subcommand with the standard config/app lifecycle.
// flagSpecs (name, default, usage) configure job-specific flags.
func addJob(parent *cobra.Command, use, short string, factory func(*app.App) gcjobs.Runner, flagSpecs ...string) {
	var configPath string
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
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
			return factory(a).Run(cmd.Context())
		},
	}
	cmd.Flags().StringVarP(&configPath, "config", "c", "config.yaml", "path to config file")
	for i := 0; i+2 < len(flagSpecs); i += 3 {
		flag, def, usage := flagSpecs[i], flagSpecs[i+1], flagSpecs[i+2]
		cmd.Flags().Duration(flag, parseDurationOrZero(def), usage)
	}
	parent.AddCommand(cmd)
}

func parseDurationOrZero(s string) time.Duration {
	if s == "" || s == "0" || s == "0s" {
		return 0
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0
	}
	return d
}
