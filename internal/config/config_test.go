package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mandacode-labs/mdrive/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadFromPath(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
app:
  name: test-app
  env: production

http:
  host: 127.0.0.1
  port: 3000
  openAPIPath: /api/openapi.yaml

database:
  driver: postgres
  host: localhost
  port: 5432
  name: testdb
  user: testuser
  sslMode: disable

cache:
  provider: valkey
  valkey:
    addr: "valkey:6379"
    db: 1
    poolSize: 20
    password: secret

storage:
  provider: s3
  region: ap-northeast-2
  bucket: test-bucket

auth:
  keycloak:
    baseURL: https://idp.example.com
    realm: test-realm
    clientID: test-client
    redirectURI: https://api.example.com/auth/callback
  session:
    ttl: 3600
    secure: true
`
	err := os.WriteFile(cfgPath, []byte(yamlContent), 0644)
	require.NoError(t, err)

	cfg, err := config.LoadFromPath(cfgPath)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Check loaded values
	assert.Equal(t, "test-app", cfg.App.Name)
	assert.Equal(t, "production", cfg.App.Env)
	assert.Equal(t, "127.0.0.1", cfg.HTTP.Host)
	assert.Equal(t, 3000, cfg.HTTP.Port)

	assert.Equal(t, "postgres", cfg.Database.Driver)
	assert.Equal(t, "localhost", cfg.Database.Host)
	assert.Equal(t, 5432, cfg.Database.Port)
	assert.Equal(t, "testdb", cfg.Database.Name)
	assert.Equal(t, "testuser", cfg.Database.User)

	assert.Equal(t, "valkey", cfg.Cache.Provider)
	assert.Equal(t, "valkey:6379", cfg.Cache.Valkey.Addr)
	assert.Equal(t, 1, cfg.Cache.Valkey.DB)
	assert.Equal(t, 20, cfg.Cache.Valkey.PoolSize)
	assert.Equal(t, "secret", cfg.Cache.Valkey.Password)

	assert.Equal(t, "s3", cfg.Storage.Provider)
	assert.Equal(t, "ap-northeast-2", cfg.Storage.Region)
	assert.Equal(t, "test-bucket", cfg.Storage.Bucket)

	assert.Equal(t, "https://idp.example.com", cfg.Auth.Keycloak.BaseURL)
	assert.Equal(t, "test-realm", cfg.Auth.Keycloak.Realm)
	assert.Equal(t, "test-client", cfg.Auth.Keycloak.ClientID)
	assert.Equal(t, "https://api.example.com/auth/callback", cfg.Auth.Keycloak.RedirectURI)
	assert.Equal(t, 3600, cfg.Auth.Session.TTL)
	assert.True(t, cfg.Auth.Session.Secure)
}

func TestLoadFromPath_Defaults(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
database:
  host: localhost
  name: testdb
  sslMode: disable

storage:
  bucket: test-bucket
`
	err := os.WriteFile(cfgPath, []byte(yamlContent), 0644)
	require.NoError(t, err)

	cfg, err := config.LoadFromPath(cfgPath)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Check defaults
	assert.Equal(t, "mdrive", cfg.App.Name)
	assert.Equal(t, "development", cfg.App.Env)
	assert.Equal(t, "0.0.0.0", cfg.HTTP.Host)
	assert.Equal(t, 8080, cfg.HTTP.Port)
	assert.Equal(t, "postgres", cfg.Database.Driver)
	assert.Equal(t, "valkey", cfg.Cache.Provider)
	assert.Equal(t, "localhost:6379", cfg.Cache.Valkey.Addr)
	assert.Equal(t, "s3", cfg.Storage.Provider)
	assert.Equal(t, "us-east-1", cfg.Storage.Region)
	assert.Equal(t, 86400, cfg.Auth.Session.TTL)
	assert.False(t, cfg.Auth.Session.Secure)

	// Telemetry defaults
	assert.False(t, cfg.Telemetry.Enabled)
	assert.Equal(t, "localhost:4317", cfg.Telemetry.Endpoint)
	assert.True(t, cfg.Telemetry.Insecure)
	assert.Equal(t, "mdrive", cfg.Telemetry.ServiceName)
}

func TestLoadFromPath_DatabasePasswordFromEnv(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
database:
  driver: postgres
  host: localhost
  port: 5432
  name: testdb
  user: testuser
  sslMode: disable

storage:
  bucket: test-bucket
`
	err := os.WriteFile(cfgPath, []byte(yamlContent), 0644)
	require.NoError(t, err)

	// Set environment variable
	require.NoError(t, os.Setenv("DATABASE_PASSWORD", "secret-password-123"))
	t.Cleanup(func() { require.NoError(t, os.Unsetenv("DATABASE_PASSWORD")) })

	cfg, err := config.LoadFromPath(cfgPath)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify password is read from environment variable
	assert.Equal(t, "secret-password-123", cfg.Database.Password)

	// Verify DSN includes the password
	expectedDSN := "host=localhost port=5432 user=testuser password=secret-password-123 dbname=testdb sslmode=disable"
	assert.Equal(t, expectedDSN, cfg.DSN())
}

func TestLoadFromPath_ValkeyPasswordFromEnv(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
database:
  host: localhost
  name: testdb
  sslMode: disable

cache:
  provider: valkey
  valkey:
    addr: valkey:6379

storage:
  bucket: test-bucket
`
	err := os.WriteFile(cfgPath, []byte(yamlContent), 0644)
	require.NoError(t, err)

	// Set environment variable
	require.NoError(t, os.Setenv("CACHE_VALKEY_PASSWORD", "valkey-secret"))
	t.Cleanup(func() { require.NoError(t, os.Unsetenv("CACHE_VALKEY_PASSWORD")) })

	cfg, err := config.LoadFromPath(cfgPath)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify password is read from environment variable
	assert.Equal(t, "valkey-secret", cfg.Cache.Valkey.Password)
}

func TestLoadFromPath_StorageCredentialsFromEnv(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
database:
  host: localhost
  name: testdb
  sslMode: disable

storage:
  provider: s3
  bucket: test-bucket
`
	err := os.WriteFile(cfgPath, []byte(yamlContent), 0644)
	require.NoError(t, err)

	// Set environment variables
	require.NoError(t, os.Setenv("STORAGE_ACCESS_KEY", "access123"))
	require.NoError(t, os.Setenv("STORAGE_SECRET_KEY", "secret456"))
	t.Cleanup(func() {
		require.NoError(t, os.Unsetenv("STORAGE_ACCESS_KEY"))
		require.NoError(t, os.Unsetenv("STORAGE_SECRET_KEY"))
	})

	cfg, err := config.LoadFromPath(cfgPath)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "access123", cfg.Storage.AccessKey)
	assert.Equal(t, "secret456", cfg.Storage.SecretKey)
}

func TestLoadFromPath_KeycloakSecretFromEnv(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
database:
  host: localhost
  name: testdb
  sslMode: disable

storage:
  bucket: test-bucket

auth:
  keycloak:
    baseURL: https://idp.example.com
    realm: test
    clientID: test-client
`
	err := os.WriteFile(cfgPath, []byte(yamlContent), 0644)
	require.NoError(t, err)

	// Set environment variable
	require.NoError(t, os.Setenv("AUTH_KEYCLOAK_CLIENT_SECRET", "kc-secret"))
	t.Cleanup(func() { require.NoError(t, os.Unsetenv("AUTH_KEYCLOAK_CLIENT_SECRET")) })

	cfg, err := config.LoadFromPath(cfgPath)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "kc-secret", cfg.Auth.Keycloak.ClientSecret)
}

func TestConfig_IsDevelopment(t *testing.T) {
	cfg := &config.Config{
		App: config.AppConfig{
			Env: "development",
		},
	}
	assert.True(t, cfg.IsDevelopment())
	assert.False(t, cfg.IsProduction())
}

func TestConfig_IsProduction(t *testing.T) {
	cfg := &config.Config{
		App: config.AppConfig{
			Env: "production",
		},
	}
	assert.False(t, cfg.IsDevelopment())
	assert.True(t, cfg.IsProduction())
}

func TestValidate_MissingDatabaseHost(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
database:
  name: testdb
  sslMode: disable

storage:
  bucket: test-bucket
`
	err := os.WriteFile(cfgPath, []byte(yamlContent), 0644)
	require.NoError(t, err)

	_, err = config.LoadFromPath(cfgPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database host")
}

func TestValidate_MissingDatabaseName(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
database:
  host: localhost

storage:
  bucket: test-bucket
`
	err := os.WriteFile(cfgPath, []byte(yamlContent), 0644)
	require.NoError(t, err)

	_, err = config.LoadFromPath(cfgPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database name")
}

func TestValidate_MissingStorageBucket(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
database:
  host: localhost
  name: testdb
`
	err := os.WriteFile(cfgPath, []byte(yamlContent), 0644)
	require.NoError(t, err)

	_, err = config.LoadFromPath(cfgPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "storage bucket")
}

func TestValidate_InvalidHTTPPort(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
http:
  port: 99999

database:
  host: localhost
  name: testdb

storage:
  bucket: test-bucket
`
	err := os.WriteFile(cfgPath, []byte(yamlContent), 0644)
	require.NoError(t, err)

	_, err = config.LoadFromPath(cfgPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP port")
}
