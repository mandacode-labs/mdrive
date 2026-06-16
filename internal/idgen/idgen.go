// Package idgen provides identifier generation utilities.
package idgen

import "github.com/oklog/ulid/v2"

// Generate returns a new ULID string.
// ULIDs are 26 characters, lexicographically sortable by creation time,
// and collision-resistant (80 bits of randomness).
func Generate() string {
	return ulid.Make().String()
}
