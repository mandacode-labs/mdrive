// Package migrate implements the `mdrive migrate` CLI subcommand.
package migrate

import (
	"context"
	"fmt"
	"os"

	"ariga.io/atlas-go-sdk/atlasexec"
	"github.com/spf13/cobra"

	"github.com/mandacode-labs/mdrive/internal/config"
)

// migrationsDir is the embedded Atlas migration directory.
var migrationsDir = os.DirFS("ent/migrate/migrations")

// NewCmd creates the `migrate` subcommand.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Manage database migrations",
	}
	cmd.AddCommand(newApplyCmd())
	return cmd
}

func newApplyCmd() *cobra.Command {
	var configPath string
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply pending Atlas versioned migrations",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.LoadFromPath(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			return apply(cmd.Context(), cfg.Database.DSN())
		},
	}
	cmd.Flags().StringVarP(&configPath, "config", "c", "config.yaml", "path to config file")
	return cmd
}

func apply(ctx context.Context, dsn string) error {
	workDir, err := atlasexec.NewWorkingDir(
		atlasexec.WithMigrations(migrationsDir),
	)
	if err != nil {
		return fmt.Errorf("atlas working dir: %w", err)
	}
	defer func() { _ = workDir.Close() }()

	client, err := atlasexec.NewClient(workDir.Path(), "atlas")
	if err != nil {
		return fmt.Errorf("atlas client: %w", err)
	}

	if _, err := client.MigrateApply(ctx, &atlasexec.MigrateApplyParams{
		URL: dsn,
	}); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}
