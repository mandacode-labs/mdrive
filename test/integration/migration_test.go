package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mdriveBin returns the command name to invoke. If MDRIVE_BIN is
// set, runs that binary directly. Otherwise falls back to `go run
// ./cmd/mdrive` (works without a separate build step).
func mdriveBin(t *testing.T) (name string, args []string, dir string) {
	t.Helper()
	repoRoot, err := filepath.Abs("../..")
	require.NoError(t, err)
	if p := os.Getenv("MDRIVE_BIN"); p != "" {
		return p, nil, repoRoot
	}
	return "go", []string{"run", "./cmd/mdrive"}, repoRoot
}

func TestMigrateApplyRequiresDatabaseURL(t *testing.T) {
	name, prefix, dir := mdriveBin(t)
	cmd := exec.Command(name, append(prefix, "migrate", "apply")...)
	cmd.Dir = dir

	out, err := cmd.CombinedOutput()
	require.Error(t, err, "migrate apply without --database-url must fail")

	assert.Contains(t, string(out), "database-url",
		"error should mention the missing flag, got: %s", string(out))
}

func TestMigrateApplyInvalidDSNFails(t *testing.T) {
	name, prefix, dir := mdriveBin(t)
	cmd := exec.Command(name, append(prefix,
		"migrate", "apply",
		"--database-url", "postgres://nobody@127.0.0.1:1/nope",
	)...)
	cmd.Dir = dir

	out, err := cmd.CombinedOutput()
	require.Error(t, err, "migrate apply with unreachable DB should fail")

	combined := strings.ToLower(string(out))
	hasMigrationErr := strings.Contains(combined, "apply migrations") ||
		strings.Contains(combined, "migrate")
	hasConnErr := strings.Contains(combined, "dial") ||
		strings.Contains(combined, "connect") ||
		strings.Contains(combined, "refused") ||
		strings.Contains(combined, "connection")
	assert.True(t, hasMigrationErr || hasConnErr,
		"error should be parseable (migration/connection), got: %s", string(out))
}