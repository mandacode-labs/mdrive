// Package migrations contains the embedded migration files.
package migrations

import "embed"

// Files holds the embedded migration SQL files and the atlas sum file.
//
//go:embed *.sql atlas.sum
var Files embed.FS
