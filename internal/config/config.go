// Package config provides application configuration loading.
package config

import (
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config is the root configuration struct.
type Config struct {
	App      AppConfig      `mapstructure:"app"`
	HTTP     HTTPConfig     `mapstructure:"http"`
	Database DatabaseConfig `mapstructure:"database"`
	Storage  StorageConfig  `mapstructure:"storage"`
	Crypto   CryptoConfig   `mapstructure:"crypto"`
	Valkey   ValkeyConfig   `mapstructure:"valkey"`
	Auth     AuthConfig     `mapstructure:"auth"`
	OpenFGA  OpenFGAConfig  `mapstructure:"openfga"`
}

// AppConfig holds application-level settings.
type AppConfig struct {
	Env      string `mapstructure:"env"`
	LogLevel string `mapstructure:"log_level"`
}

// HTTPConfig holds HTTP server settings.
type HTTPConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

// DatabaseConfig holds PostgreSQL settings.
type DatabaseConfig struct {
	Driver   string `mapstructure:"driver"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Name     string `mapstructure:"name"`
	SSLMode  string `mapstructure:"sslmode"`
}

// DSN returns the PostgreSQL connection string.
func (c DatabaseConfig) DSN() string {
	return strings.Join([]string{
		"host=" + c.Host,
		"port=" + itoa(c.Port),
		"user=" + c.User,
		"password=" + c.Password,
		"dbname=" + c.Name,
		"sslmode=" + c.SSLMode,
	}, " ")
}

func itoa(i int) string {
	return strings.TrimSpace(string(rune('0' + i)))
}

// StorageConfig holds S3/object-storage settings.
type StorageConfig struct {
	Region       string `mapstructure:"region"`
	Endpoint     string `mapstructure:"endpoint"`
	AccessKey    string `mapstructure:"access_key"`
	SecretKey    string `mapstructure:"secret_key"`
	UsePathStyle bool   `mapstructure:"use_path_style"`
	PresignTTL   string `mapstructure:"presign_ttl"`
}

// PresignTTL returns the presigned URL TTL as a time.Duration.
func (c StorageConfig) PresignTTLDuration() time.Duration {
	if c.PresignTTL == "" {
		return time.Hour
	}
	d, err := time.ParseDuration(c.PresignTTL)
	if err != nil {
		return time.Hour
	}
	return d
}

// CryptoConfig holds at-rest encryption settings.
type CryptoConfig struct {
	MasterKey string `mapstructure:"master_key"`
}

// ValkeyConfig holds Valkey/Redis connection settings.
type ValkeyConfig struct {
	Addrs    []string `mapstructure:"addrs"`
	Password string   `mapstructure:"password"`
	DB       int      `mapstructure:"db"`
	TLS      bool     `mapstructure:"tls"`
}

// AuthConfig holds authentication settings.
type AuthConfig struct {
	Provider string `mapstructure:"provider"` // "zitadel", "keycloak"
	Issuer   string `mapstructure:"issuer"`
	ClientID string `mapstructure:"client_id"`
	JWKSURL  string `mapstructure:"jwks_url"`
}

// OpenFGAConfig holds OpenFGA settings.
type OpenFGAConfig struct {
	APIURL               string `mapstructure:"api_url"`
	StoreID              string `mapstructure:"store_id"`
	AuthorizationModelID string `mapstructure:"authorization_model_id"`
}

// Load reads the configuration from the given path.
func Load(path string) (*Config, error) {
	return LoadFromPath(path)
}

// LoadFromPath reads the configuration from the given file path.
func LoadFromPath(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	setDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("app.env", "development")
	v.SetDefault("app.log_level", "info")
	v.SetDefault("http.host", "0.0.0.0")
	v.SetDefault("http.port", 8080)
	v.SetDefault("database.driver", "postgres")
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "mdrive")
	v.SetDefault("database.password", "")
	v.SetDefault("database.name", "mdrive")
	v.SetDefault("database.sslmode", "disable")
	v.SetDefault("storage.region", "us-east-1")
	v.SetDefault("storage.endpoint", "")
	v.SetDefault("storage.use_path_style", false)
	v.SetDefault("storage.presign_ttl", "1h")
	v.SetDefault("crypto.master_key", "")
	v.SetDefault("valkey.addrs", []string{"localhost:6379"})
	v.SetDefault("valkey.password", "")
	v.SetDefault("valkey.db", 0)
	v.SetDefault("valkey.tls", false)
	v.SetDefault("auth.provider", "zitadel")
	v.SetDefault("openfga.api_url", "http://localhost:8081")
}
