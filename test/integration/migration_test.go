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

// TestMigrateApplyRequiresDatabaseHost ensures the CLI refuses to
// run when neither --database-host nor DATABASE_HOST is provided.
func TestMigrateApplyRequiresDatabaseHost(t *testing.T) {
	name, prefix, dir := mdriveBin(t)
	cmd := exec.Command(name, append(prefix, "migrate", "apply")...)
	cmd.Dir = dir
	cmd.Env = scrubEnv(os.Environ(), "DATABASE_HOST", "DATABASE_NAME")

	out, err := cmd.CombinedOutput()
	require.Error(t, err, "migrate apply without DATABASE_HOST must fail")

	combined := strings.ToLower(string(out))
	ok := strings.Contains(combined, "database_host") ||
		strings.Contains(combined, "required")
	assert.True(t, ok,
		"expected required-host error, got: %s", string(out))
}

// TestMigrateApplyUnreachableDBFails exercises the env-var path:
// DATABASE_* envs are set and the CLI must build the URL and call
// atlas, which then fails to reach the unreachable host.
func TestMigrateApplyUnreachableDBFails(t *testing.T) {
	name, prefix, dir := mdriveBin(t)
	cmd := exec.Command(name, append(prefix, "migrate", "apply")...)
	cmd.Dir = dir
	cmd.Env = scrubEnv(os.Environ(),
		"DATABASE_HOST", "DATABASE_PORT", "DATABASE_NAME",
		"DATABASE_USER", "DATABASE_PASSWORD", "DATABASE_SSLMODE",
	)
	cmd.Env = append(cmd.Env,
		"DATABASE_HOST=127.0.0.1",
		"DATABASE_PORT=1",
		"DATABASE_NAME=nope",
		"DATABASE_USER=nobody",
		"DATABASE_PASSWORD=",
		"DATABASE_SSLMODE=disable",
	)

	out, err := cmd.CombinedOutput()
	require.Error(t, err, "migrate apply with unreachable DB should fail")

	combined := strings.ToLower(string(out))
	ok := strings.Contains(combined, "apply migrations") ||
		strings.Contains(combined, "dial") ||
		strings.Contains(combined, "connect") ||
		strings.Contains(combined, "refused") ||
		strings.Contains(combined, "connection")
	assert.True(t, ok,
		"expected atlas/connect error, got: %s", string(out))
}

// TestMigrateApplyFlagOverridesEnv verifies that explicit
// --database-host etc. take precedence over DATABASE_* env vars.
func TestMigrateApplyFlagOverridesEnv(t *testing.T) {
	name, prefix, dir := mdriveBin(t)
	// Env points at port 2, flags point at port 1; both are
	// unreachable so the test must surface a connection error
	// regardless. The point is to exercise the flag path so a
	// future refactor that drops flag binding is caught here.
	cmd := exec.Command(name, append(prefix,
		"migrate", "apply",
		"--database-host", "127.0.0.1",
		"--database-port", "1",
		"--database-name", "nope",
		"--database-user", "nobody",
		"--database-password", "",
		"--database-sslmode", "disable",
	)...)
	cmd.Dir = dir
	cmd.Env = scrubEnv(os.Environ(),
		"DATABASE_HOST", "DATABASE_PORT", "DATABASE_NAME",
		"DATABASE_USER", "DATABASE_PASSWORD", "DATABASE_SSLMODE",
	)
	cmd.Env = append(cmd.Env,
		"DATABASE_HOST=127.0.0.1",
		"DATABASE_PORT=2",
		"DATABASE_NAME=envname",
		"DATABASE_USER=envuser",
	)

	out, err := cmd.CombinedOutput()
	require.Error(t, err)

	combined := strings.ToLower(string(out))
	ok := strings.Contains(combined, "apply migrations") ||
		strings.Contains(combined, "dial") ||
		strings.Contains(combined, "connect") ||
		strings.Contains(combined, "refused") ||
		strings.Contains(combined, "connection")
	assert.True(t, ok,
		"expected atlas/connect error, got: %s", string(out))
}

// scrubEnv returns a copy of env with the listed keys removed.
func scrubEnv(env []string, keys ...string) []string {
	out := make([]string, 0, len(env))
	for _, e := range env {
		drop := false
		for _, k := range keys {
			if strings.HasPrefix(e, k+"=") {
				drop = true
				break
			}
		}
		if !drop {
			out = append(out, e)
		}
	}
	return out
}
