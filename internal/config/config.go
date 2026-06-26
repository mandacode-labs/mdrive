// Package config provides application configuration loading.
package config

import (
	"errors"
	"net/http"
	"os"
	"strconv"
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
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	IdleTimeout     time.Duration `mapstructure:"idle_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
	CORS            CORSConfig    `mapstructure:"cors"`
	Cookie          CookieConfig  `mapstructure:"cookie"`
}

// CORSConfig holds Cross-Origin Resource Sharing settings.
type CORSConfig struct {
	Enabled          bool     `mapstructure:"enabled"`
	AllowedOrigins   []string `mapstructure:"allowed_origins"`
	AllowedMethods   []string `mapstructure:"allowed_methods"`
	AllowedHeaders   []string `mapstructure:"allowed_headers"`
	ExposedHeaders   []string `mapstructure:"exposed_headers"`
	AllowCredentials bool     `mapstructure:"allow_credentials"`
	MaxAge           int      `mapstructure:"max_age"`
}

// CookieConfig holds session cookie settings.
type CookieConfig struct {
	Name     string        `mapstructure:"name"`
	Path     string        `mapstructure:"path"`
	Secure   bool          `mapstructure:"secure"`
	HttpOnly bool          `mapstructure:"http_only"`
	SameSite string        `mapstructure:"same_site"`
	TTL      time.Duration `mapstructure:"ttl"`
}

// SameSiteMode returns the http.SameSite value for the
// configured same_site string. Lax is the default.
func (c CookieConfig) SameSiteMode() http.SameSite {
	switch strings.ToLower(c.SameSite) {
	case "strict":
		return http.SameSiteStrictMode
	case "lax", "":
		return http.SameSiteLaxMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
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
		"port=" + strconv.Itoa(c.Port),
		"user=" + c.User,
		"password=" + c.Password,
		"dbname=" + c.Name,
		"sslmode=" + c.SSLMode,
	}, " ")
}

// StorageConfig holds S3/object-storage settings.
type StorageConfig struct {
	Region       string        `mapstructure:"region"`
	Endpoint     string        `mapstructure:"endpoint"`
	Bucket       string        `mapstructure:"bucket"`
	AccessKey    string        `mapstructure:"access_key"`
	SecretKey    string        `mapstructure:"secret_key"`
	UsePathStyle bool          `mapstructure:"use_path_style"`
	PresignTTL   time.Duration `mapstructure:"presign_ttl"`
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
	Provider    string        `mapstructure:"provider"` // "zitadel", "keycloak"
	Issuer      string        `mapstructure:"issuer"`
	ClientID    string        `mapstructure:"client_id"`
	SessionTTL  time.Duration `mapstructure:"session_ttl"`
	FrontendURL string        `mapstructure:"frontend_url"`
}

// OpenFGAConfig holds OpenFGA settings.
// Secret fields can be set via env vars:
//
//	OPENFGA_API_TOKEN, OPENFGA_CLIENT_ID, OPENFGA_CLIENT_SECRET
//
// AuthMode selects the credential mode and is validated at startup:
//   - "api_token"           : requires api_token
//   - "client_credentials"  : requires client_id, client_secret, token_issuer, audience
//   - "none"                : no credentials (development only)
type OpenFGAConfig struct {
	AuthMode             string `mapstructure:"auth_mode"`
	APIURL               string `mapstructure:"api_url"`
	StoreID              string `mapstructure:"store_id"`
	AuthorizationModelID string `mapstructure:"authorization_model_id"`
	APIToken             string `mapstructure:"api_token"`
	ClientID             string `mapstructure:"client_id"`
	ClientSecret         string `mapstructure:"client_secret"`
	TokenIssuer          string `mapstructure:"token_issuer"`
	Audience             string `mapstructure:"audience"`
}

// LoadFromPath reads the configuration from the given file path.
// Call Validate(env) afterwards to enforce production invariants.
//
// The config file is optional. When the path does not exist (or no
// path is set and viper's search-path lookup finds nothing), the
// loader falls back to viper defaults and AutomaticEnv env vars
// (viper's SetEnvKeyReplacer(\".\", \"_\") means DATABASE_HOST env
// populates the database.host key, etc.). This lets callers run
// with env-only config — e.g. the migration Job in helm, where
// the ConfigMap is not yet created at PreSync time.
//
// Surface real read errors (parse failure, permission denied) so
// the caller can distinguish \"no file\" from \"bad file\".
func LoadFromPath(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	setDefaults(v)

	if rcErr := v.ReadInConfig(); rcErr != nil {
		// Missing config is fine: defaults + env vars populate the
		// config. viper surfaces this two ways depending on how the
		// path was set:
		//   - SetConfigFile with a specific path -> underlying
		//     *fs.PathError wrapping os.ErrNotExist
		//   - empty path with search lookup -> viper.ConfigFileNotFoundError
		isMissing := errors.Is(rcErr, os.ErrNotExist)
		if !isMissing {
			var notFound viper.ConfigFileNotFoundError
			isMissing = errors.As(rcErr, &notFound)
		}
		if !isMissing {
			return nil, rcErr
		}
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
	v.SetDefault("http.read_timeout", "30s")
	v.SetDefault("http.write_timeout", "30s")
	v.SetDefault("http.idle_timeout", "120s")
	v.SetDefault("http.shutdown_timeout", "30s")
	v.SetDefault("http.cors.enabled", true)
	v.SetDefault("http.cors.allowed_origins", []string{"*"})
	v.SetDefault("http.cors.allowed_methods", []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"})
	v.SetDefault("http.cors.allowed_headers", []string{"Authorization", "Content-Type", "X-Requested-With"})
	v.SetDefault("http.cors.exposed_headers", []string{"Content-Length"})
	v.SetDefault("http.cors.allow_credentials", true)
	v.SetDefault("http.cors.max_age", 86400)
	v.SetDefault("http.cookie.name", "mdrive_session")
	v.SetDefault("http.cookie.path", "/")
	v.SetDefault("http.cookie.secure", false)
	v.SetDefault("http.cookie.http_only", true)
	v.SetDefault("http.cookie.same_site", "lax")
	v.SetDefault("http.cookie.ttl", "24h")
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
	v.SetDefault("storage.bucket", "mdrive")
	v.SetDefault("storage.presign_ttl", "1h")
	v.SetDefault("crypto.master_key", "")
	v.SetDefault("valkey.addrs", []string{"localhost:6379"})
	v.SetDefault("valkey.password", "")
	v.SetDefault("valkey.db", 0)
	v.SetDefault("valkey.tls", false)
	v.SetDefault("auth.provider", "zitadel")
	v.SetDefault("auth.issuer", "")
	v.SetDefault("auth.client_id", "")
	v.SetDefault("auth.session_ttl", "24h")
	v.SetDefault("auth.frontend_url", "http://localhost:3000")
	v.SetDefault("openfga.auth_mode", "api_token")
	v.SetDefault("openfga.api_url", "")
	v.SetDefault("openfga.store_id", "")
	v.SetDefault("openfga.authorization_model_id", "")
	v.SetDefault("openfga.api_token", "")
	v.SetDefault("openfga.client_id", "")
	v.SetDefault("openfga.client_secret", "")
	v.SetDefault("openfga.token_issuer", "")
	v.SetDefault("openfga.audience", "")
}

// Validate enforces production-vs-development invariants. Returns
// nil if the config is sound for the given environment; otherwise
// an error describing the first violation.
func (c *Config) Validate(env string) error {
	isProd := env != "development"
	if isProd {
		if c.Crypto.MasterKey == "" {
			return errProductionMasterKeyRequired
		}
		if c.OpenFGA.APIURL == "" {
			return errProductionOpenFGARequired
		}
	}
	return nil
}

var (
	errProductionMasterKeyRequired = errors.New("config: crypto.master_key is required in production")
	errProductionOpenFGARequired   = errors.New("config: openfga.api_url is required in production")
)
