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

// TestOpenFGAScopesEmptyDefault verifies that openfga.scopes
// defaults to empty (no implicit value). Operators must set it
// explicitly when auth_mode=client_credentials.
func TestOpenFGAScopesEmptyDefault(t *testing.T) {
	for _, k := range []string{"OPENFGA_SCOPES"} {
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
	assert.Equal(t, "", cfg.OpenFGA.Scopes,
		"openfga.scopes should default to empty; do not silently set 'openid' (different IdPs require different scopes)")
}

// TestValidateRequiresScopesForClientCredentials verifies the
// production fail-fast: client_credentials mode without scopes
// must be rejected at startup. RFC 6749 §4.4 requires the scope
// parameter; OIDC providers reject scope-less requests.
func TestValidateRequiresScopesForClientCredentials(t *testing.T) {
	cfg := &Config{
		Crypto: CryptoConfig{MasterKey: "x"},
		OpenFGA: OpenFGAConfig{
			APIURL:   "http://openfga:8080",
			AuthMode: "client_credentials",
			Scopes:   "",
		},
	}
	err := cfg.Validate("production")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scopes")

	// Setting scopes makes Validate pass (other production
	// invariants are already satisfied).
	cfg.OpenFGA.Scopes = "openid"
	assert.NoError(t, cfg.Validate("production"))
}

// TestValidateScopesOptionalInDevelopment verifies that the
// scopes requirement is a production-only invariant. In dev
// mode (env=development) the empty scopes default is allowed.
func TestValidateScopesOptionalInDevelopment(t *testing.T) {
	cfg := &Config{
		OpenFGA: OpenFGAConfig{
			APIURL:   "http://openfga:8080",
			AuthMode: "client_credentials",
			Scopes:   "",
		},
	}
	assert.NoError(t, cfg.Validate("development"),
		"empty scopes should be tolerated in development")
}

// TestAuthEncryptionKeyEnvOverride verifies that AUTH_ENCRYPTION_KEY
// env var (the name the Helm chart deploys) overrides the YAML
// value. PR-61 introduced encryption_key wiring; without BindEnv,
// viper's AutomaticEnv would only see CONFIG_AUTH_ENCRYPTION_KEY
// (the dot-notation-mapped name), so the chart-injected env is
// silently dropped — exactly what the production rollout exposed.
func TestAuthEncryptionKeyEnvOverride(t *testing.T) {
	for _, k := range []string{"AUTH_ENCRYPTION_KEY", "CONFIG_AUTH_ENCRYPTION_KEY"} {
		_ = os.Unsetenv(k)
	}

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(`
app:
  env: production
database:
  driver: postgres
  host: localhost
  port: 5432
  user: mdrive
  password: ""
  name: mdrive
  sslmode: disable
auth:
  issuer: https://sso.example.com
  client_id: client-123
`), 0o600))

	t.Setenv("AUTH_ENCRYPTION_KEY", "env-key-32-chars-long-aaaaaaa")

	cfg, err := LoadFromPath(cfgPath)
	require.NoError(t, err)
	assert.Equal(t, "env-key-32-chars-long-aaaaaaa", cfg.Auth.EncryptionKey,
		"AUTH_ENCRYPTION_KEY env must bind to cfg.Auth.EncryptionKey (chart injects this name)")
}

// TestOpenFGASecretKeyEnvOverride verifies the same env-binding pattern
// for every other secret the chart wires via Secret references:
// DATABASE_PASSWORD, VALKEY_PASSWORD, CRYPTO_MASTER_KEY, OPENFGA_*.
// Catches the same class of viper mapping bug for all of them at once.
func TestOpenFGASecretKeyEnvOverride(t *testing.T) {
	for _, k := range []string{
		"DATABASE_PASSWORD", "VALKEY_PASSWORD", "CRYPTO_MASTER_KEY",
		"OPENFGA_API_TOKEN", "OPENFGA_CLIENT_ID", "OPENFGA_CLIENT_SECRET",
	} {
		_ = os.Unsetenv(k)
	}

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(`
app:
  env: production
database:
  driver: postgres
  host: localhost
  port: 5432
  user: mdrive
  password: ""
  name: mdrive
  sslmode: disable
valkey:
  password: ""
openfga:
  api_token: ""
  client_id: ""
  client_secret: ""
`), 0o600))

	t.Setenv("DATABASE_PASSWORD", "db-env-pass")
	t.Setenv("VALKEY_PASSWORD", "vk-env-pass")
	t.Setenv("CRYPTO_MASTER_KEY", "crypto-env-key")
	t.Setenv("OPENFGA_API_TOKEN", "ofg-api-token")
	t.Setenv("OPENFGA_CLIENT_ID", "ofg-client-id")
	t.Setenv("OPENFGA_CLIENT_SECRET", "ofg-client-secret")

	cfg, err := LoadFromPath(cfgPath)
	require.NoError(t, err)
	assert.Equal(t, "db-env-pass", cfg.Database.Password)
	assert.Equal(t, "vk-env-pass", cfg.Valkey.Password)
	assert.Equal(t, "crypto-env-key", cfg.Crypto.MasterKey)
	assert.Equal(t, "ofg-api-token", cfg.OpenFGA.APIToken)
	assert.Equal(t, "ofg-client-id", cfg.OpenFGA.ClientID)
	assert.Equal(t, "ofg-client-secret", cfg.OpenFGA.ClientSecret)
}