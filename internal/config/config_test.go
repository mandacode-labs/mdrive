package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoadFromPathMissingFallsBackToEnv verifies that the config
// loader tolerates a missing config file. The migration Job in
// helm runs with no --config arg (defaults to "config.yaml" in
// CWD), and the ConfigMap it would otherwise read is not yet
// created at PreSync time — the file is genuinely absent.
func TestLoadFromPathMissingFallsBackToEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DATABASE_HOST", "env-host")
	t.Setenv("DATABASE_PORT", "5433")
	t.Setenv("DATABASE_NAME", "env-db")
	t.Setenv("DATABASE_USER", "env-user")
	t.Setenv("DATABASE_PASSWORD", "env-pass")
	t.Setenv("DATABASE_SSLMODE", "disable")
	t.Setenv("APP_ENV", "development")

	cfg, err := LoadFromPath(filepath.Join(dir, "does-not-exist.yaml"))
	require.NoError(t, err, "missing config file should be tolerated")
	require.NotNil(t, cfg)

	assert.Equal(t, "env-host", cfg.Database.Host)
	assert.Equal(t, 5433, cfg.Database.Port)
	assert.Equal(t, "env-db", cfg.Database.Name)
	assert.Equal(t, "env-user", cfg.Database.User)
	assert.Equal(t, "env-pass", cfg.Database.Password)
	assert.Equal(t, "disable", cfg.Database.SSLMode)
	assert.Contains(t, cfg.Database.DSN(), "host=env-host")
}

// TestLoadFromPathEmptyPathWithNoSearch verifies that an empty
// path (which puts viper in search-path mode) is also tolerated
// when the search returns nothing — viper returns a typed
// ConfigFileNotFoundError here, not a fs.PathError.
func TestLoadFromPathEmptyPathWithNoSearch(t *testing.T) {
	t.Setenv("DATABASE_HOST", "env-host")
	t.Setenv("DATABASE_PORT", "5433")
	t.Setenv("DATABASE_NAME", "env-db")
	t.Setenv("DATABASE_USER", "env-user")
	t.Setenv("DATABASE_PASSWORD", "env-pass")
	t.Setenv("DATABASE_SSLMODE", "disable")
	t.Setenv("APP_ENV", "development")

	cfg, err := LoadFromPath("")
	require.NoError(t, err, "empty-path / no-search should be tolerated")
	require.NotNil(t, cfg)
	assert.Equal(t, "env-host", cfg.Database.Host)
}

// TestLoadFromPathInvalidYAMLReturnsError verifies that a real read
// error (bad YAML) is surfaced so the caller can distinguish
// "no file" from "bad file".
func TestLoadFromPathInvalidYAMLReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("invalid: [unclosed"), 0o600))

	_, err := LoadFromPath(path)
	require.Error(t, err, "invalid YAML must surface an error")
	assert.NotContains(t, err.Error(), "no such file")
}