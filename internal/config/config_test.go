package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValkeyUserDefault verifies that the implicit "default" user
// is used when the config has no user field set. This matches
// Redis 6 and earlier behaviour (no ACL) and keeps existing
// single-user deployments working on upgrade.
func TestValkeyUserDefault(t *testing.T) {
	// Clear any VALKEY_* env that could pollute the test.
	for _, k := range []string{"VALKEY_USER", "VALKEY_ADDRS", "VALKEY_PASSWORD"} {
		_ = os.Unsetenv(k)
	}

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(`
app:
  env: development
database:
  driver: postgres
  host: localhost
  port: 5432
  user: mdrive
  password: ""
  name: mdrive
  sslmode: disable
`), 0o600))

	cfg, err := LoadFromPath(cfgPath)
	require.NoError(t, err)
	assert.Equal(t, "default", cfg.Valkey.User,
		"valkey.user should default to \"default\" for backward compatibility")
}

// TestValkeyUserYAMLOverride verifies that an explicit
// valkey.user in YAML overrides the default.
func TestValkeyUserYAMLOverride(t *testing.T) {
	for _, k := range []string{"VALKEY_USER"} {
		_ = os.Unsetenv(k)
	}

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(`
app:
  env: development
database:
  driver: postgres
  host: localhost
  port: 5432
  user: mdrive
  password: ""
  name: mdrive
  sslmode: disable
valkey:
  user: mdrive
`), 0o600))

	cfg, err := LoadFromPath(cfgPath)
	require.NoError(t, err)
	assert.Equal(t, "mdrive", cfg.Valkey.User)
}

// TestValkeyUserEnvOverride verifies that VALKEY_USER env var
// overrides the YAML value. This is the path the Helm chart
// exercises when it injects VALKEY_USER via the deployment env.
func TestValkeyUserEnvOverride(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(`
app:
  env: development
database:
  driver: postgres
  host: localhost
  port: 5432
  user: mdrive
  password: ""
  name: mdrive
  sslmode: disable
valkey:
  user: yaml-user
`), 0o600))

	t.Setenv("VALKEY_USER", "env-user")
	cfg, err := LoadFromPath(cfgPath)
	require.NoError(t, err)
	assert.Equal(t, "env-user", cfg.Valkey.User,
		"VALKEY_USER env var should win over YAML value (viper AutomaticEnv)")
}