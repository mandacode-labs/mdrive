// Package migrate implements the `mdrive migrate` CLI subcommand.
package migrate

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	"ariga.io/atlas-go-sdk/atlasexec"
	"github.com/spf13/cobra"

	"github.com/mandacode-labs/mdrive/internal/cliflags"
	"github.com/mandacode-labs/mdrive/internal/config"
)

//go:embed migrations/*.sql migrations/atlas.sum
var defaultMigrations embed.FS

const defaultAtlasBin = "atlas"

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
	cliflags.AddConfigFlag(cmd, &configPath)
	return cmd
}

func apply(ctx context.Context, dsn string) error {
	migrations, err := fs.Sub(defaultMigrations, "migrations")
	if err != nil {
		return fmt.Errorf("migrations subdir: %w", err)
	}
	return applyWith(ctx, dsn, migrations, defaultAtlasBin)
}

func applyWith(ctx context.Context, dsn string, migrations fs.FS, atlasBin string) error {
	workDir, err := atlasexec.NewWorkingDir(atlasexec.WithMigrations(migrations))
	if err != nil {
		return fmt.Errorf("atlas working dir: %w", err)
	}
	defer func() { _ = workDir.Close() }()

	client, err := atlasexec.NewClient(workDir.Path(), atlasBin)
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
