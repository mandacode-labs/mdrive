package migrations

import "embed"

// FS bundles the embedded atlas migrations and their hash file.
// The application uses this for boot-time schema verification
// (app/schema.go) and the cli migrate subcommand uses it for
// `migrate apply` (cli/migrate).
//
//go:embed *.sql atlas.sum
var FS embed.FS