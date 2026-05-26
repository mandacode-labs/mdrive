package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application.
type Config struct {
	App       AppConfig       `mapstructure:"app" yaml:"app"`
	HTTP      HTTPConfig      `mapstructure:"http" yaml:"http"`
	Database  DatabaseConfig  `mapstructure:"database" yaml:"database"`
	Cache     CacheConfig     `mapstructure:"cache" yaml:"cache"`
	Storage   StorageConfig   `mapstructure:"storage" yaml:"storage"`
	Auth      AuthConfig      `mapstructure:"auth" yaml:"auth"`
	CORS      CORSConfig      `mapstructure:"cors" yaml:"cors"`
	Telemetry TelemetryConfig `mapstructure:"telemetry" yaml:"telemetry"`
}

// AppConfig holds application-level configuration.
type AppConfig struct {
	Name    string `mapstructure:"name" yaml:"name"`
	Version string `mapstructure:"version" yaml:"version"`
	Env     string `mapstructure:"env" yaml:"env"`
}

// HTTPConfig holds HTTP server configuration.
type HTTPConfig struct {
	Host string `mapstructure:"host" yaml:"host"`
	Port int    `mapstructure:"port" yaml:"port"`
}

// DatabaseConfig holds database connection configuration.
type DatabaseConfig struct {
	Driver   string `mapstructure:"driver" yaml:"driver"`
	Host     string `mapstructure:"host" yaml:"host"`
	Port     int    `mapstructure:"port" yaml:"port"`
	Name     string `mapstructure:"name" yaml:"name"`
	User     string `mapstructure:"user" yaml:"user"`
	Password string `mapstructure:"password" yaml:"password"`
	SSLMode  string `mapstructure:"sslMode" yaml:"sslMode"`
}

// DSN returns the database connection string.
func (c DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode,
	)
}

// CacheConfig holds cache configuration.
type CacheConfig struct {
	Provider string       `mapstructure:"provider" yaml:"provider"`
	Valkey   ValkeyConfig `mapstructure:"valkey" yaml:"valkey"`
}

// ValkeyConfig holds Valkey connection configuration.
type ValkeyConfig struct {
	Addr     string `mapstructure:"addr" yaml:"addr"`
	DB       int    `mapstructure:"db" yaml:"db"`
	PoolSize int    `mapstructure:"poolSize" yaml:"poolSize"`
	Username string `mapstructure:"username" yaml:"username"`
	Password string `mapstructure:"password" yaml:"password"`
}

// StorageConfig holds storage backend configuration.
type StorageConfig struct {
	Provider  string `mapstructure:"provider" yaml:"provider"`
	Region    string `mapstructure:"region" yaml:"region"`
	Endpoint  string `mapstructure:"endpoint" yaml:"endpoint"`
	AccessKey string `mapstructure:"accessKey" yaml:"accessKey"`
	SecretKey string `mapstructure:"secretKey" yaml:"secretKey"`
	Bucket    string `mapstructure:"bucket" yaml:"bucket"`
	Prefix    string `mapstructure:"prefix" yaml:"prefix"`
	UseSSL    bool   `mapstructure:"useSSL" yaml:"useSSL"`
}

// StorageKeyPrefix returns the configured prefix with trailing slash, or empty string.
func (s *StorageConfig) StorageKeyPrefix() string {
	if s.Prefix == "" {
		return ""
	}
	return strings.TrimRight(s.Prefix, "/") + "/"
}

// AuthConfig holds authentication configuration.
type AuthConfig struct {
	Keycloak KeycloakConfig `mapstructure:"keycloak" yaml:"keycloak"`
	Session  SessionConfig  `mapstructure:"session" yaml:"session"`
}

// KeycloakConfig holds Keycloak configuration.
type KeycloakConfig struct {
	BaseURL      string `mapstructure:"baseURL" yaml:"baseURL"`
	Realm        string `mapstructure:"realm" yaml:"realm"`
	ClientID     string `mapstructure:"clientID" yaml:"clientID"`
	ClientSecret string `mapstructure:"clientSecret" yaml:"clientSecret"`
	RedirectURI  string `mapstructure:"redirectURI" yaml:"redirectURI"`
}

// SessionConfig holds session configuration.
type SessionConfig struct {
	TTL         int    `mapstructure:"ttl" yaml:"ttl"`                 // Session TTL in seconds
	Secure      bool   `mapstructure:"secure" yaml:"secure"`           // Set Secure flag on cookie
	StateTTL    int    `mapstructure:"stateTTL" yaml:"stateTTL"`       // OAuth state TTL in seconds
	RedisKey    string `mapstructure:"redisKey" yaml:"redisKey"`       // Redis key prefix
	CookieName  string `mapstructure:"cookieName" yaml:"cookieName"`   // Session cookie name
	FrontendURL string `mapstructure:"frontendURL" yaml:"frontendURL"` // Frontend URL for redirect after login
	Domain      string `mapstructure:"domain" yaml:"domain"`           // Cookie domain (e.g., ".starfrag.co" for cross-subdomain)
	SameSite    string `mapstructure:"sameSite" yaml:"sameSite"`       // SameSite policy: "lax", "strict", "none"
}

// TelemetryConfig holds OpenTelemetry configuration.
type TelemetryConfig struct {
	Enabled     bool   `mapstructure:"enabled" yaml:"enabled"`
	Endpoint    string `mapstructure:"endpoint" yaml:"endpoint"`
	Insecure    bool   `mapstructure:"insecure" yaml:"insecure"`
	CACert      string `mapstructure:"caCert" yaml:"caCert"`
	ServiceName string `mapstructure:"serviceName" yaml:"serviceName"`
}

// CORSConfig holds CORS configuration.
type CORSConfig struct {
	Enabled          bool     `mapstructure:"enabled" yaml:"enabled"`
	AllowedOrigins   []string `mapstructure:"allowedOrigins" yaml:"allowedOrigins"`
	AllowedMethods   []string `mapstructure:"allowedMethods" yaml:"allowedMethods"`
	AllowedHeaders   []string `mapstructure:"allowedHeaders" yaml:"allowedHeaders"`
	ExposedHeaders   []string `mapstructure:"exposedHeaders" yaml:"exposedHeaders"`
	AllowCredentials bool     `mapstructure:"allowCredentials" yaml:"allowCredentials"`
	MaxAge           int      `mapstructure:"maxAge" yaml:"maxAge"` // in seconds
}

// OIDCRedirectURI returns the OIDC redirect URI.
func (c *KeycloakConfig) OIDCRedirectURI(baseURL string) string {
	return baseURL + "/auth/callback"
}

// DSN returns the database connection string.
func (c *Config) DSN() string {
	return c.Database.DSN()
}

// IsDevelopment returns true if running in development mode.
func (c *Config) IsDevelopment() bool {
	return strings.ToLower(c.App.Env) == "development"
}

// IsProduction returns true if running in production mode.
func (c *Config) IsProduction() bool {
	return strings.ToLower(c.App.Env) == "production"
}

// Load reads configuration from file and environment variables.
func Load(configPath string) (*Config, error) {
	v := viper.New()

	setDefaults(v)
	bindEnvVars(v)

	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

// LoadFromPath reads configuration from a specific path.
func LoadFromPath(configPath string) (*Config, error) {
	return Load(configPath)
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("app.name", "retrowin")
	v.SetDefault("app.version", "0.1.0")
	v.SetDefault("app.env", "development")
	v.SetDefault("http.host", "0.0.0.0")
	v.SetDefault("http.port", 8080)
	v.SetDefault("database.driver", "postgres")
	v.SetDefault("database.sslMode", "disable")
	v.SetDefault("cache.provider", "valkey")
	v.SetDefault("cache.valkey.addr", "localhost:6379")
	v.SetDefault("cache.valkey.db", 0)
	v.SetDefault("cache.valkey.poolSize", 10)
	v.SetDefault("cache.valkey.password", "")
	v.SetDefault("storage.provider", "s3")
	v.SetDefault("storage.region", "us-east-1")
	v.SetDefault("storage.useSSL", false)
	v.SetDefault("auth.session.ttl", 86400) // 24 hours
	v.SetDefault("auth.session.secure", false)
	v.SetDefault("auth.session.stateTTL", 300) // 5 minutes
	v.SetDefault("auth.session.redisKey", "retrowin")
	v.SetDefault("auth.session.cookieName", "session_id")
	v.SetDefault("auth.session.frontendURL", "http://localhost:5173")
	v.SetDefault("auth.session.domain", "")      // Empty means current host only
	v.SetDefault("auth.session.sameSite", "lax") // lax, strict, or none
	v.SetDefault("auth.keycloak.redirectURI", "http://localhost:8080/auth/callback")
	v.SetDefault("telemetry.enabled", false)
	v.SetDefault("telemetry.endpoint", "localhost:4317")
	v.SetDefault("telemetry.insecure", true)
	v.SetDefault("telemetry.serviceName", "retrowin")
}

func bindEnvVars(v *viper.Viper) {
	// Support nested env vars like DATABASE_PASSWORD, CACHE_VALKEY_PASSWORD
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Explicitly bind common secrets (camelCase config keys)
	_ = v.BindEnv("database.password", "DATABASE_PASSWORD")
	_ = v.BindEnv("cache.valkey.password", "CACHE_VALKEY_PASSWORD")
	_ = v.BindEnv("storage.accessKey", "STORAGE_ACCESS_KEY")
	_ = v.BindEnv("storage.secretKey", "STORAGE_SECRET_KEY")
	_ = v.BindEnv("auth.keycloak.clientSecret", "AUTH_KEYCLOAK_CLIENT_SECRET")
	_ = v.BindEnv("telemetry.enabled", "TELEMETRY_ENABLED")
	_ = v.BindEnv("telemetry.endpoint", "TELEMETRY_ENDPOINT")
}
