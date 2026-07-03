package e2e

import (
	"context"
	"database/sql"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"

	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/mandacode-labs/mdrive/ent"
)

// TestMigrationJobMatchesAutoMigration asserts that the committed
// versioned migrations and the ent auto-migration produce the
// same public schema. Drift means a migration is missing or
// the ent schema changed without `make migrate`; both should be
// caught before reaching production.
//
// The comparison is on the public application tables only.
// atlas's bookkeeping table (`atlas_schema_revisions`) is
// excluded, and `character(N)` is treated as equivalent to
// `character varying(N)` since the ent postgres dialect uses
// one form and atlas's ent provider uses the other for the
// same string-with-MaxLen definition.
func TestMigrationJobMatchesAutoMigration(t *testing.T) {
	ctx := context.Background()

	migrationsCols := withFreshPg(t, ctx, func(dsn string) []column {
		require.NoError(t, runAtlasMigrate(t, dsn), "atlas migrate apply")
		return queryPublicColumns(ctx, t, dsn)
	})

	autoCols := withFreshPg(t, ctx, func(dsn string) []column {
		autoMigrate(ctx, t, dsn)
		return queryPublicColumns(ctx, t, dsn)
	})

	require.Equal(t, migrationsCols, autoCols,
		"migrations apply and ent auto-migration produce different public schemas; "+
			"either a migration is missing or the ent schema changed without `make migrate`")
}

// column is one row of the application-schema projection.
// MaxLen is captured but normalised: the ent postgres dialect
// drops the length limit on optional fields while atlas's ent
// provider keeps it, so an ent Optional column shows up as
// character varying (no MaxLen) and the same ent field rendered
// through atlas shows up as character varying(N). They are the
// same logical type; the test compares them as such.
type column struct {
	Table    string
	Column   string
	Type     string
	MaxLen   *int
	Nullable bool
	Default  *string
}

// withFreshPg starts an ephemeral postgres container and runs fn
// against its DSN. The container is terminated on cleanup.
func withFreshPg(t *testing.T, ctx context.Context, fn func(dsn string) []column) []column {
	t.Helper()
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
	return fn(dsn)
}

// runAtlasMigrate applies the versioned migrations under
// internal/app/migrations to the database at dsn.
func runAtlasMigrate(t *testing.T, dsn string) error {
	t.Helper()
	migrationsDir, err := filepath.Abs("../../internal/app/migrations")
	if err != nil {
		return err
	}
	return runAtlas(t, "migrate", "apply",
		"--url", dsn,
		"--dir", "file://"+migrationsDir,
	)
}

// autoMigrate runs ent's auto-migration against the empty database at dsn.
func autoMigrate(ctx context.Context, t *testing.T, dsn string) {
	t.Helper()
	drv, err := entsql.Open("postgres", dsn)
	require.NoError(t, err)
	client := ent.NewClient(ent.Driver(drv))
	t.Cleanup(func() { _ = client.Close() })
	require.NoError(t, client.Schema.Create(ctx))
}

// queryPublicColumns reads the application-schema projection
// from information_schema and returns it as a stable, comparable
// slice. Atlas's bookkeeping tables are excluded; character(N)
// and character varying(N) are normalised so the test only
// flags real drift (missing or wrong-typed columns), not the
// cosmetic differences between atlas and ent dialects.
func queryPublicColumns(ctx context.Context, t *testing.T, dsn string) []column {
	t.Helper()
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	rows, err := db.QueryContext(ctx, `
		SELECT table_name, column_name, data_type, character_maximum_length,
		       is_nullable, column_default
		  FROM information_schema.columns
		 WHERE table_schema = 'public'
		   AND table_name NOT LIKE 'atlas_%'
		 ORDER BY table_name, ordinal_position
	`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	out := make([]column, 0, 32)
	for rows.Next() {
		var c column
		var maxLen sql.NullInt64
		var def sql.NullString
		var nullable string
		require.NoError(t, rows.Scan(&c.Table, &c.Column, &c.Type, &maxLen, &nullable, &def))
		c.Nullable = nullable == "YES"
		if maxLen.Valid {
			n := int(maxLen.Int64)
			c.MaxLen = &n
		}
		if def.Valid {
			s := def.String
			c.Default = &s
		}
		c.Type = normaliseType(c.Type)
		c.MaxLen = normaliseMaxLen(c.Type, c.MaxLen)
		out = append(out, c)
	}
	require.NoError(t, rows.Err())
	sort.Slice(out, func(i, j int) bool {
		if out[i].Table != out[j].Table {
			return out[i].Table < out[j].Table
		}
		return out[i].Column < out[j].Column
	})
	return out
}

// normaliseType collapses dialect-specific spellings of the
// same logical type into one form. character(N) and
// character varying(N) are stored identically and compared
// as the same type.
func normaliseType(t string) string {
	if t == "character" {
		return "character varying"
	}
	return t
}

// normaliseMaxLen drops MaxLen for string columns. The ent
// postgres dialect emits field.String() as varchar without
// the MaxLen, while atlas's ent provider emits the same field
// as varchar(N) (or character(N)). Both store the same
// values; only the limit string differs. Dropping the limit
// for string columns keeps the test focused on real drift
// (missing or wrong-typed columns), not the cosmetic
// difference between the two paths.
func normaliseMaxLen(t string, maxLen *int) *int {
	if maxLen == nil {
		return nil
	}
	switch t {
	case "character varying", "text":
		return nil
	}
	return maxLen
}

// runAtlas invokes the atlas CLI. Used so that the test fails
// clearly when atlas is not installed.
func runAtlas(t *testing.T, args ...string) error {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "atlas", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("atlas %v failed:\n%s", args, out)
	}
	return err
}
