// Package migrate implements the migrate command
package migrate

import (
	"github.com/spf13/cobra"

	"github.com/mandacode-labs/retrowin-go/internal/config"
)

// NewCmd creates a new cobra command for the migrate command
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Database migration commands",
	}

	cmd.AddCommand(newApplyCmd())

	return cmd
}

// newApplyCmd creates the migrate apply subcommand
func newApplyCmd() *cobra.Command {
	var cfgFile string
	var mode string
	var baseline string
	var clean bool
	var allowDirty bool

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply database migrations",
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

			// CLI flags override config
			opts := MigrateOptions{
				Mode:       mode,
				Baseline:   baseline,
				Clean:      clean,
				AllowDirty: allowDirty,
			}
			if opts.Mode == "" {
				opts.Mode = "auto"
			}

			return ApplyMigrations(cfg, opts)
		},
	}

	cmd.Flags().StringVarP(&cfgFile, "config", "c", "", "config file path")
	cmd.Flags().StringVar(&mode, "mode", "", "Migration mode: auto or versioned (overrides config)")
	cmd.Flags().StringVar(&baseline, "baseline", "", "Baseline version for existing databases (versioned mode)")
	cmd.Flags().BoolVar(&clean, "clean", false, "Drop all tables before applying migrations")
	cmd.Flags().BoolVar(&allowDirty, "allow-dirty", false, "Allow applying versioned migrations to a non-clean database")

	return cmd
}
