//go:build integration

package migrate

import (
	"context"
	"database/sql"
	"testing"
	"testing/fstest"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestApplyVersionedMigration(t *testing.T) {
	ctx := context.Background()

	pg, err := postgres.Run(ctx, "postgres:17-alpine",
		postgres.WithDatabase("mdrive"),
		postgres.WithUsername("mdrive"),
		postgres.WithPassword("mdrive"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pg.Terminate(ctx) })

	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	// First apply should create all schema objects.
	err = apply(ctx, dsn)
	require.NoError(t, err)

	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	expectedTables := []string{"users", "drives", "drive_storage", "nodes", "gc_tombstones"}
	for _, table := range expectedTables {
		var exists bool
		err := db.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT FROM information_schema.tables
				WHERE table_schema = 'public' AND table_name = $1
			)
		`, table).Scan(&exists)
		require.NoError(t, err, "check table %s", table)
		require.True(t, exists, "table %s should exist", table)
	}

	// Second apply should be idempotent.
	err = apply(ctx, dsn)
	require.NoError(t, err)
}

func TestApplyWithInvalidDSN(t *testing.T) {
	err := applyWith(context.Background(), "invalid://dsn", defaultMigrations, defaultAtlasBin)
	require.Error(t, err)
}

func TestApplyWithEmptyMigrations(t *testing.T) {
	ctx := context.Background()

	pg, err := postgres.Run(ctx, "postgres:17-alpine",
		postgres.WithDatabase("mdrive"),
		postgres.WithUsername("mdrive"),
		postgres.WithPassword("mdrive"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pg.Terminate(ctx) })

	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	// Empty migrations directory: atlas reports success with nothing applied.
	err = applyWith(ctx, dsn, fstest.MapFS{}, defaultAtlasBin)
	require.NoError(t, err)

	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	var exists bool
	err = db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = $1
		)
	`, "users").Scan(&exists)
	require.NoError(t, err)
	require.False(t, exists, "users table should not exist when no migrations are provided")
}
