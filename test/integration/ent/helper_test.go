//go:build integration_ent

package ent

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/mandacode-labs/mdrive/ent"
)

// startPostgres brings up a real Postgres in a testcontainer,
// applies the ent schema in-process, and returns an *ent.Client
// pointed at it. The container is terminated on t.Cleanup.
//
// The schema is created from the in-process ent definitions,
// not from the production migration files. The migration files
// are covered by internal/cli/migrate/migrate_test.go; this
// helper only needs a usable Postgres with the mdrive tables
// present so the ent repositories can be exercised.
func startPostgres(t *testing.T) *ent.Client {
	t.Helper()
	ctx := context.Background()

	pg, err := postgres.Run(ctx, "postgres:17-alpine",
		postgres.WithDatabase("mdrive"),
		postgres.WithUsername("mdrive"),
		postgres.WithPassword("mdrive"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err, "start postgres container")
	t.Cleanup(func() { _ = pg.Terminate(ctx) })

	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	drv := entsql.OpenDB(dialect.Postgres, db)
	client := ent.NewClient(ent.Driver(drv))
	t.Cleanup(func() { _ = client.Close() })

	require.NoError(t, client.Schema.Create(ctx), "create ent schema")
	return client
}
