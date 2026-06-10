// Package migrate implements the migrate command
package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"entgo.io/ent/dialect/sql/schema"

	"ariga.io/atlas/atlasexec"
	"github.com/mandacode-labs/mdrive/ent"
	"github.com/mandacode-labs/mdrive/ent/migrate/migrations"
	"github.com/mandacode-labs/mdrive/internal/config"
	"github.com/mandacode-labs/mdrive/internal/logging"
)

// MigrateOptions holds options for migration apply.
type MigrateOptions struct {
	Mode       string // "auto" or "versioned"
	Baseline   string // Baseline version for existing databases (versioned mode only)
	Clean      bool   // Drop all tables before applying
	AllowDirty bool   // Allow applying versioned migrations to a non-clean database
}

// ApplyMigrations applies database migrations using the configured mode.
func ApplyMigrations(cfg *config.Config, opts MigrateOptions, logger *logging.Logger) error {
	if opts.Clean {
		if err := dropAllTables(cfg); err != nil {
			return fmt.Errorf("failed to drop tables: %w", err)
		}
		logger.Info().Msg("all tables dropped")
	}

	mode := opts.Mode
	if mode == "" {
		mode = "auto" // default
	}

	switch mode {
	case "versioned":
		return applyVersionedMigrations(cfg, opts, logger)
	case "auto", "":
		return applyAutoMigrations(cfg, logger)
	default:
		return fmt.Errorf("unknown migration mode: %s (use 'auto' or 'versioned')", mode)
	}
}

// dropAllTables drops all tables in the public schema using CASCADE.
func dropAllTables(cfg *config.Config) error {
	db, err := sql.Open(cfg.Database.Driver, cfg.DSN())
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() { _ = db.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Query all table names in the public schema and drop them all at once
	var query string
	switch cfg.Database.Driver {
	case "postgres":
		query = `
				SELECT COALESCE(string_agg('DROP TABLE IF EXISTS ' || quote_ident(table_schema) || '.' || quote_ident(table_name) || ' CASCADE', E';\n'), '')
				FROM information_schema.tables
				WHERE table_schema = 'public' AND table_type = 'BASE TABLE'`
	default:
		return fmt.Errorf("unsupported database driver for clean: %s", cfg.Database.Driver)
	}

	var stmts string
	if err := db.QueryRowContext(ctx, query).Scan(&stmts); err != nil {
		return fmt.Errorf("failed to build drop statements: %w", err)
	}
	if stmts == "" {
		return nil
	}

	_, err = db.ExecContext(ctx, stmts)
	return err
}

// applyAutoMigrations uses ent's auto-migration (Schema.Create).
func applyAutoMigrations(cfg *config.Config, logger *logging.Logger) error {
	entClient, err := ent.Open(cfg.Database.Driver, cfg.DSN())
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() { _ = entClient.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := entClient.Schema.Create(ctx, schema.WithGlobalUniqueID(true)); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	logger.Info().Msg("auto-migrations applied successfully")
	return nil
}

// applyVersionedMigrations uses atlasexec to apply versioned SQL migrations.
// Requires the 'atlas' CLI binary to be available in PATH.
func applyVersionedMigrations(cfg *config.Config, opts MigrateOptions, logger *logging.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Create a working directory with embedded migration files
	workdir, err := atlasexec.NewWorkingDir(
		atlasexec.WithMigrations(migrations.Files),
	)
	if err != nil {
		return fmt.Errorf("failed to create atlas working directory: %w", err)
	}
	defer func() { _ = workdir.Close() }()

	// Create atlas client
	client, err := atlasexec.NewClient(workdir.Path(), "atlas")
	if err != nil {
		return fmt.Errorf("failed to create atlas client: %w", err)
	}

	// Build database URL from config
	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.Database.User, cfg.Database.Password,
		cfg.Database.Host, cfg.Database.Port,
		cfg.Database.Name, cfg.Database.SSLMode,
	)

	// Apply pending migrations
	params := &atlasexec.MigrateApplyParams{
		URL: dbURL,
	}
	if opts.Baseline != "" {
		params.BaselineVersion = opts.Baseline
	}
	if opts.AllowDirty {
		params.AllowDirty = true
	}

	res, err := client.MigrateApply(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to apply versioned migrations: %w", err)
	}

	if len(res.Applied) == 0 {
		logger.Info().Msg("no pending migrations")
	} else {
		for _, f := range res.Applied {
			logger.Info().Str("migration", f.Name).Msg("applied migration")
		}
		logger.Info().
			Int("count", len(res.Applied)).
			Str("from", res.Current).
			Str("to", res.Target).
			Msg("versioned migrations applied successfully")
	}

	return nil
}
