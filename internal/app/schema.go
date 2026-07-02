package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/logx"
)

// requiredColumn is one (table, column) the application expects
// to find in the live database, with a substring match on the
// PostgreSQL data type. The list is hardcoded so the boot check
// stays self-contained — no atlas / docker / ent dependency
// at startup, and no need to parse CREATE TABLE SQL at runtime.
//
// When the ent schema changes, update this list in the same
// commit as the migration; the boot check is the safety net
// that catches the case where the migration is forgotten.
type requiredColumn struct {
	Table string
	Name  string
	Type  string
}

var requiredColumns = []requiredColumn{
	// nodes
	{"nodes", "id", "uuid"},
	{"nodes", "type", "character varying"},
	{"nodes", "size", "bigint"},
	{"nodes", "nlink", "bigint"},
	{"nodes", "mode", "bigint"},
	{"nodes", "uid", "character"},
	{"nodes", "gid", "character"},
	{"nodes", "content", "bytea"},
	{"nodes", "atime", "timestamp"},
	{"nodes", "mtime", "timestamp"},
	{"nodes", "ctime", "timestamp"},
	{"nodes", "crtime", "timestamp"},
	{"nodes", "flags", "bigint"},
	{"nodes", "revision", "character"},
	{"nodes", "create_time", "timestamp"},
	{"nodes", "update_time", "timestamp"},

	// drives
	{"drives", "id", "character"},
	{"drives", "public_id", "character"},
	{"drives", "name", "character"},
	{"drives", "description", "character"},
	{"drives", "provider", "character"},
	{"drives", "owner_id", "character"},
	{"drives", "root_node_id", "uuid"},
	{"drives", "deleted_at", "timestamp"},
	{"drives", "create_time", "timestamp"},
	{"drives", "update_time", "timestamp"},

	// drive_storage
	{"drive_storage", "id", "uuid"},
	{"drive_storage", "drive_id", "character"},
	{"drive_storage", "bucket", "character"},
	{"drive_storage", "endpoint", "character"},
	{"drive_storage", "region", "character"},
	{"drive_storage", "access_key", "character"},
	{"drive_storage", "secret_key", "bytea"},
	{"drive_storage", "use_path_style", "boolean"},
	{"drive_storage", "create_time", "timestamp"},
	{"drive_storage", "update_time", "timestamp"},

	// users
	{"users", "id", "character"},
	{"users", "public_id", "character"},
	{"users", "name", "character"},
	{"users", "email", "character"},
	{"users", "create_time", "timestamp"},
	{"users", "update_time", "timestamp"},
}

// verifySchema reads information_schema.columns and confirms
// every required (table, column) is present with a matching
// type. Production fails-fast on drift so the operator sees
// the schema problem at startup rather than as a silent 5xx
// in production traffic. Development logs a warning and
// continues so the existing auto-create behavior is
// preserved for local work.
//
// Returns nil on success or a KindServiceDegraded error on
// drift in production.
func verifySchema(ctx context.Context, db *sql.DB, env string) error {
	for _, c := range requiredColumns {
		var name, dataType string
		err := db.QueryRowContext(ctx,
			`SELECT column_name, data_type FROM information_schema.columns
			 WHERE table_name = $1 AND column_name = $2`,
			c.Table, c.Name,
		).Scan(&name, &dataType)
		if err == sql.ErrNoRows {
			logx.Warn(ctx, "schema.missing_column",
				slog.String("table", c.Table),
				slog.String("column", c.Name),
				slog.String("env", env),
			)
			if env == "production" {
				return errorx.New(errorx.KindServiceDegraded,
					fmt.Sprintf("schema: missing column %s.%s", c.Table, c.Name))
			}
			continue
		}
		if err != nil {
			return errorx.Wrap(err, fmt.Sprintf("schema: query %s.%s", c.Table, c.Name))
		}
		if !strings.Contains(dataType, c.Type) {
			logx.Warn(ctx, "schema.wrong_type",
				slog.String("table", c.Table),
				slog.String("column", c.Name),
				slog.String("expected", c.Type),
				slog.String("actual", dataType),
				slog.String("env", env),
			)
			if env == "production" {
				return errorx.New(errorx.KindServiceDegraded,
					fmt.Sprintf("schema: wrong type for %s.%s: expected %s, got %s",
						c.Table, c.Name, c.Type, dataType))
			}
		}
	}
	logx.Info(ctx, "schema.verified",
		slog.String("env", env),
		slog.Int("columns_checked", len(requiredColumns)),
	)
	return nil
}
