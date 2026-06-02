package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/retrowin-go/internal/cmd/migrate"
)

func TestMigration_AutoMode(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	suite := NewSuite(t)
	err := suite.Start(ctx)
	require.NoError(t, err, "Failed to start test suite")
	t.Cleanup(func() { _ = suite.Stop(ctx) })

	cfg := suite.GetConfig()

	t.Run("applies auto migrations to empty database", func(t *testing.T) {
		err := migrate.ApplyMigrations(cfg, migrate.MigrateOptions{Mode: "auto"})
		require.NoError(t, err, "Auto migration should succeed")

		// Verify core tables exist
		tables := []string{"users", "systems", "inodes", "objects"}
		for _, table := range tables {
			var exists bool
			err := suite.GetDB().QueryRow(
				"SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1)",
				table,
			).Scan(&exists)
			require.NoError(t, err)
			assert.True(t, exists, "Table %s should exist after auto migration", table)
		}
	})

	t.Run("idempotent auto migration", func(t *testing.T) {
		err := migrate.ApplyMigrations(cfg, migrate.MigrateOptions{Mode: "auto"})
		require.NoError(t, err, "Running auto migration again should succeed")
	})
}

func TestMigration_VersionedMode(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	suite := NewSuite(t)
	err := suite.Start(ctx)
	require.NoError(t, err, "Failed to start test suite")
	t.Cleanup(func() { _ = suite.Stop(ctx) })

	cfg := suite.GetConfig()

	// suite.Start() already ran auto-migrations, so the database has tables.
	// Use baseline to skip migrations that were already applied by auto-migration.
	// The baseline must be the latest migration version that exists in the database.
	t.Run("applies versioned migrations with baseline on existing database", func(t *testing.T) {
		err := migrate.ApplyMigrations(cfg, migrate.MigrateOptions{
			Mode:     "versioned",
			Baseline: "20260602021721",
		})
		require.NoError(t, err, "Versioned migration with baseline should succeed")
	})

	t.Run("no pending migrations on reapply", func(t *testing.T) {
		err := migrate.ApplyMigrations(cfg, migrate.MigrateOptions{Mode: "versioned"})
		require.NoError(t, err, "Reapplying versioned migrations should succeed")
	})
}

func TestMigration_VersionedOnCleanDB(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	suite := NewSuite(t)
	err := suite.Start(ctx)
	require.NoError(t, err, "Failed to start test suite")
	t.Cleanup(func() { _ = suite.Stop(ctx) })

	cfg := suite.GetConfig()

	t.Run("clean then versioned migration on fresh database", func(t *testing.T) {
		// Clean drops all tables (including ones created by suite.Start auto-migration)
		err := migrate.ApplyMigrations(cfg, migrate.MigrateOptions{
			Mode:  "versioned",
			Clean: true,
		})
		require.NoError(t, err, "Clean + versioned migration should succeed")

		// Verify core tables exist
		tables := []string{"users", "systems", "inodes", "objects"}
		for _, table := range tables {
			var exists bool
			err := suite.GetDB().QueryRow(
				"SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1)",
				table,
			).Scan(&exists)
			require.NoError(t, err)
			assert.True(t, exists, "Table %s should exist after versioned migration", table)
		}
	})

	t.Run("no pending migrations on reapply", func(t *testing.T) {
		err := migrate.ApplyMigrations(cfg, migrate.MigrateOptions{Mode: "versioned"})
		require.NoError(t, err, "Reapplying versioned migrations should succeed")
	})
}

func TestMigration_CleanAndReapply(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	suite := NewSuite(t)
	err := suite.Start(ctx)
	require.NoError(t, err, "Failed to start test suite")
	t.Cleanup(func() { _ = suite.Stop(ctx) })

	cfg := suite.GetConfig()

	// First apply to create tables
	err = migrate.ApplyMigrations(cfg, migrate.MigrateOptions{Mode: "auto"})
	require.NoError(t, err, "Initial migration should succeed")

	// Verify tables exist
	var tableCount int
	err = suite.GetDB().QueryRow(
		"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_type = 'BASE TABLE'",
	).Scan(&tableCount)
	require.NoError(t, err)
	require.True(t, tableCount > 0, "Should have tables after initial migration")

	t.Run("clean drops all tables", func(t *testing.T) {
		err := migrate.ApplyMigrations(cfg, migrate.MigrateOptions{Mode: "auto", Clean: true})
		require.NoError(t, err, "Clean migration should succeed")

		// Verify tables were recreated
		var newCount int
		err = suite.GetDB().QueryRow(
			"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_type = 'BASE TABLE'",
		).Scan(&newCount)
		require.NoError(t, err)
		assert.True(t, newCount > 0, "Tables should be recreated after clean+apply")
	})

	t.Run("clean on empty database is safe", func(t *testing.T) {
		// Drop everything first
		err := migrate.ApplyMigrations(cfg, migrate.MigrateOptions{Clean: true})
		require.NoError(t, err, "Clean on empty database should not error")

		// Re-apply
		err = migrate.ApplyMigrations(cfg, migrate.MigrateOptions{Mode: "auto"})
		require.NoError(t, err)
	})
}
