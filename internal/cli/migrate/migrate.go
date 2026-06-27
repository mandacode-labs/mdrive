// Package migrate implements the `mdrive migrate` CLI subcommand.
package migrate

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	"ariga.io/atlas-go-sdk/atlasexec"
	"github.com/spf13/cobra"
)

//go:embed migrations/*.sql migrations/atlas.sum
var defaultMigrations embed.FS

const defaultAtlasBin = "atlas"

func NewCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "migrate",
		Short: "Manage database migrations",
	}
	root.AddCommand(newApplyCmd())
	return root
}

func newApplyCmd() *cobra.Command {
	var databaseURL string
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply pending Atlas versioned migrations",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return apply(cmd.Context(), databaseURL)
		},
	}
	cmd.Flags().StringVar(&databaseURL, "database-url", "",
		"PostgreSQL URL (e.g. postgres://user:password@host:5432/dbname?sslmode=require)")
	_ = cmd.MarkFlagRequired("database-url")
	return cmd
}

func apply(ctx context.Context, databaseURL string) error {
	migrations, err := fs.Sub(defaultMigrations, "migrations")
	if err != nil {
		return fmt.Errorf("migrations subdir: %w", err)
	}
	return applyWith(ctx, databaseURL, migrations, defaultAtlasBin)
}

func applyWith(ctx context.Context, databaseURL string, migrations fs.FS, atlasBin string) error {
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
		URL: databaseURL,
	}); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}