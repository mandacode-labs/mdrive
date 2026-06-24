//go:build integration_ent

package ent

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// mustParseID parses a UUID string for use in a subsequent Get.
// Kept in its own file so the test files do not need to
// repeat the require/import boilerplate.
func mustParseID(t *testing.T, s string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(s)
	require.NoError(t, err, "parse uuid %q", s)
	return id
}
