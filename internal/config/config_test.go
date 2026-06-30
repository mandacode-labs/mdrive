package config

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

// TestValidateRejectsInvalidAESKeySize verifies that production
// fails fast when the chart-injected encryption_key is not 16/24/32
// bytes (the only sizes aes.NewCipher accepts).
func TestValidateRejectsInvalidAESKeySize(t *testing.T) {
	build := func(keyLen int) *Config {
		return &Config{
			App:    AppConfig{Env: "production"},
			Crypto: CryptoConfig{MasterKey: "x"},
			Auth:   AuthConfig{EncryptionKey: strings.Repeat("a", keyLen)},
			OpenFGA: OpenFGAConfig{
				APIURL:   "http://openfga:8080",
				AuthMode: "api_token",
				APIToken: "x",
			},
		}
	}
	assert.NoError(t, build(16).Validate("production"), "16 bytes is a valid AES-128 key")
	assert.NoError(t, build(24).Validate("production"), "24 bytes is a valid AES-192 key")
	assert.NoError(t, build(32).Validate("production"), "32 bytes is a valid AES-256 key")
	for _, n := range []int{8, 15, 17, 31, 33, 64} {
		assert.Error(t, build(n).Validate("production"),
			"%d bytes is not a valid AES key size", n)
	}
}

// TestAuthScopesDefault verifies that the OIDC standard scopes
// (openid, profile, email) are set when the config does not
// override them. The default lives in config.setDefaults so the
// chart can leave config.auth.scopes unset.
func TestAuthScopesDefault(t *testing.T) {
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
auth:
  issuer: https://sso.example.com
  client_id: client-123
`), 0o600))

	cfg, err := LoadFromPath(cfgPath)
	require.NoError(t, err)
	assert.Equal(t, []string{"openid", "profile", "email"}, cfg.Auth.Scopes,
		"OIDC standard scopes must default to openid/profile/email")
}

// TestAuthScopesYAMLOverride verifies that an explicit config.auth.scopes
// value wins over the default. Real-world use: add a custom scope
// like \"roles\" so it appears in the access token.
func TestAuthScopesYAMLOverride(t *testing.T) {
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
auth:
  issuer: https://sso.example.com
  client_id: client-123
  scopes:
    - openid
    - profile
    - roles
`), 0o600))

	cfg, err := LoadFromPath(cfgPath)
	require.NoError(t, err)
	assert.Equal(t, []string{"openid", "profile", "roles"}, cfg.Auth.Scopes,
		"YAML scopes override must replace the default")
}
// SameSite are wired from the YAML tree (chart values.yaml) into
// the struct via mapstructure, and that SameSiteMode parses the
// string into the http.SameSite enum.
func TestHTTPCookieFields(t *testing.T) {
	for _, k := range []string{"COOKIE_DOMAIN", "COOKIE_SAME_SITE"} {
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
http:
  cookie:
    domain: ".mdrive.mandacode.com"
    same_site: "strict"
`), 0o600))

	cfg, err := LoadFromPath(cfgPath)
	require.NoError(t, err)
	assert.Equal(t, ".mdrive.mandacode.com", cfg.HTTP.Cookie.Domain,
		"Cookie.Domain must come from config.http.cookie.domain")
	assert.Equal(t, http.SameSiteStrictMode, cfg.HTTP.Cookie.SameSiteMode(),
		"SameSiteMode must parse config.http.cookie.same_site into http.SameSite")
}

// TestAuthEncryptionKeyEnvOverride verifies that chart-injected
// AUTH_ENCRYPTION_KEY wins over YAML's empty default. PR-61 added
// encryption_key wiring; without the post-Unmarshal re-read, viper's
// Unmarshal drops the env value (YAML's "" shadows it). Production
// rollout failed exactly this way until the override was added.
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
	assert.Equal(t, "env-key-32-chars-long-aaaaaaa", cfg.Auth.EncryptionKey)
}

// TestOpenFGASecretKeyEnvOverride verifies the same env-binding pattern
// for every other secret the chart wires via Secret references.
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