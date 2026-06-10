package e2e

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	valkeymodule "github.com/testcontainers/testcontainers-go/modules/valkey"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/valkey-io/valkey-go"
	"gopkg.in/yaml.v3"

	openapispec "github.com/mandacode-labs/mdrive/api"
	"github.com/mandacode-labs/mdrive/ent"
	"github.com/mandacode-labs/mdrive/internal/cmd/serve"
	"github.com/mandacode-labs/mdrive/internal/config"
	"github.com/mandacode-labs/mdrive/internal/logging"
)

// Shared containers — started once in TestMain, reused by all tests.
var (
	sharedOnce       sync.Once
	sharedPg         *postgres.PostgresContainer
	sharedValkey     *valkeymodule.ValkeyContainer
	sharedMinio      testcontainers.Container
	sharedPgHost     string
	sharedPgPort     int
	sharedValkeyAddr string
	sharedMinioAddr  string
)

// startSharedContainers starts PostgreSQL, Valkey, and MinIO containers once.
func startSharedContainers(ctx context.Context) error {
	var startErr error
	sharedOnce.Do(func() {
		// Start PostgreSQL
		pgContainer, err := postgres.Run(ctx, "postgres:17-alpine",
			postgres.WithDatabase("postgres"),
			postgres.WithUsername("test"),
			postgres.WithPassword("test"),
			testcontainers.WithWaitStrategy(
				wait.ForLog("database system is ready to accept connections").
					WithOccurrence(2).
					WithStartupTimeout(30*time.Second),
			),
		)
		if err != nil {
			startErr = fmt.Errorf("failed to start postgres: %w", err)
			return
		}
		sharedPg = pgContainer

		pgHost, err := pgContainer.Host(ctx)
		if err != nil {
			startErr = fmt.Errorf("failed to get postgres host: %w", err)
			return
		}
		pgPort, err := pgContainer.MappedPort(ctx, "5432")
		if err != nil {
			startErr = fmt.Errorf("failed to get postgres port: %w", err)
			return
		}
		sharedPgHost = pgHost
		sharedPgPort = int(pgPort.Num())

		// Start Valkey
		valkeyContainer, err := valkeymodule.Run(ctx, "valkey/valkey:8-alpine",
			testcontainers.WithWaitStrategy(
				wait.ForLog("Ready to accept connections").
					WithOccurrence(1).
					WithStartupTimeout(30*time.Second),
			),
		)
		if err != nil {
			startErr = fmt.Errorf("failed to start valkey: %w", err)
			return
		}
		sharedValkey = valkeyContainer

		vkHost, err := valkeyContainer.Host(ctx)
		if err != nil {
			startErr = fmt.Errorf("failed to get valkey host: %w", err)
			return
		}
		vkPort, err := valkeyContainer.MappedPort(ctx, "6379")
		if err != nil {
			startErr = fmt.Errorf("failed to get valkey port: %w", err)
			return
		}
		sharedValkeyAddr = fmt.Sprintf("%s:%s", vkHost, vkPort.Port())

		// Start MinIO
		minioContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image:        "minio/minio:latest",
				ExposedPorts: []string{"9000/tcp"},
				Env: map[string]string{
					"MINIO_ROOT_USER":     "minioadmin",
					"MINIO_ROOT_PASSWORD": "minioadmin",
				},
				Cmd: []string{"server", "/data"},
				WaitingFor: wait.ForExposedPort().
					WithStartupTimeout(30 * time.Second),
			},
			Started: true,
		})
		if err != nil {
			startErr = fmt.Errorf("failed to start minio: %w", err)
			return
		}
		sharedMinio = minioContainer

		mnHost, err := minioContainer.Host(ctx)
		if err != nil {
			startErr = fmt.Errorf("failed to get minio host: %w", err)
			return
		}
		mnPort, err := minioContainer.MappedPort(ctx, "9000")
		if err != nil {
			startErr = fmt.Errorf("failed to get minio port: %w", err)
			return
		}
		sharedMinioAddr = fmt.Sprintf("%s:%s", mnHost, mnPort.Port())

		logger := logging.NewLogger("test", "info")
		logger.Info().
			Str("pg", fmt.Sprintf("%s:%d", sharedPgHost, sharedPgPort)).
			Str("valkey", sharedValkeyAddr).
			Str("minio", sharedMinioAddr).
			Msg("shared containers started")
	})
	return startErr
}

// stopSharedContainers stops all shared containers.
func stopSharedContainers(ctx context.Context) {
	if sharedPg != nil {
		_ = testcontainers.TerminateContainer(sharedPg)
	}
	if sharedValkey != nil {
		_ = testcontainers.TerminateContainer(sharedValkey)
	}
	if sharedMinio != nil {
		_ = testcontainers.TerminateContainer(sharedMinio)
	}
}

// Suite represents an independent e2e test environment.
// Each Suite gets its own database and bucket within shared containers.
type Suite struct {
	t            *testing.T
	Config       *config.Config
	EntClient    *ent.Client
	DB           *sql.DB
	ValkeyAddr   string
	MinioAddr    string
	ValkeyClient valkey.Client
	baseURL      string
	httpClient   *http.Client
	cookieJar    []*http.Cookie
	app          *serve.App
	cfgFile      string
	dbName       string // per-test database name
	bucketName   string // per-test bucket name
}

// NewSuite creates a new independent test suite
func NewSuite(t *testing.T) *Suite {
	return &Suite{
		t: t,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		cookieJar: make([]*http.Cookie, 0),
	}
}

// Start sets up the test environment using shared containers with per-test isolation.
func (s *Suite) Start(ctx context.Context) error {
	// Ensure shared containers are running (started once by TestMain)
	if err := startSharedContainers(ctx); err != nil {
		return fmt.Errorf("failed to start shared containers: %w", err)
	}

	// Use shared container addresses
	s.ValkeyAddr = sharedValkeyAddr
	s.MinioAddr = sharedMinioAddr

	// Create Valkey client for test helpers (with retry)
	var (
		valkeyClient valkey.Client
		err          error
	)
	for i := 0; i < 10; i++ {
		valkeyClient, err = valkey.NewClient(valkey.ClientOption{
			InitAddress: []string{s.ValkeyAddr},
		})
		if err == nil {
			break
		}
		logger := logging.NewLogger("test", "info")
		logger.Warn().
			Int("attempt", i+1).
			Err(err).
			Msg("valkey client not ready, retrying")
		time.Sleep(500 * time.Millisecond)
	}
	if err != nil {
		return fmt.Errorf("failed to create valkey client after retries: %w", err)
	}
	s.ValkeyClient = valkeyClient

	// Create a per-test database for isolation (with retry)
	s.dbName = "mdrive_test_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:12]
	adminDSN := fmt.Sprintf("host=%s port=%d user=test password=test dbname=postgres sslmode=disable",
		sharedPgHost, sharedPgPort)

	var adminDB *sql.DB
	for i := 0; i < 10; i++ {
		adminDB, err = sql.Open("postgres", adminDSN)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if err = adminDB.PingContext(ctx); err != nil {
			_ = adminDB.Close()
			time.Sleep(500 * time.Millisecond)
			continue
		}
		break
	}
	if err != nil {
		return fmt.Errorf("failed to connect to admin database after retries: %w", err)
	}
	_, err = adminDB.ExecContext(ctx, fmt.Sprintf("CREATE DATABASE %s", s.dbName))
	_ = adminDB.Close()
	if err != nil {
		return fmt.Errorf("failed to create test database: %w", err)
	}

	// Connect to the per-test database
	testDSN := fmt.Sprintf("host=%s port=%d user=test password=test dbname=%s sslmode=disable",
		sharedPgHost, sharedPgPort, s.dbName)
	s.DB, err = sql.Open("postgres", testDSN)
	if err != nil {
		return fmt.Errorf("failed to open test database: %w", err)
	}

	// Create Ent client
	drv := entsql.OpenDB(dialect.Postgres, s.DB)
	s.EntClient = ent.NewClient(ent.Driver(drv))

	// Run migrations
	if err := s.EntClient.Schema.Create(ctx); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Find an available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("failed to find available port: %w", err)
	}
	testPort := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()

	// Create per-test bucket in shared MinIO
	s.bucketName = "test-bucket-" + strings.ReplaceAll(uuid.New().String(), "-", "")[:12]
	if err := s.createMinioBucket(ctx); err != nil {
		return fmt.Errorf("failed to create minio bucket: %w", err)
	}

	// Create test config
	s.Config = &config.Config{
		App: config.AppConfig{
			Name:    "mdrive-test",
			Version: "test",
			Env:     "test",
		},
		HTTP: config.HTTPConfig{
			Host: "127.0.0.1",
			Port: testPort,
		},
		Database: config.DatabaseConfig{
			Driver:   "postgres",
			Host:     sharedPgHost,
			Port:     sharedPgPort,
			Name:     s.dbName,
			User:     "test",
			Password: "test",
			SSLMode:  "disable",
		},
		Cache: config.CacheConfig{
			Provider: "valkey",
			Valkey: config.ValkeyConfig{
				Addr:     s.ValkeyAddr,
				DB:       0,
				PoolSize: 10,
				Username: "",
			},
		},
		Storage: config.StorageConfig{
			Provider:  "s3",
			Region:    "us-east-1",
			Endpoint:  "http://" + s.MinioAddr,
			AccessKey: "minioadmin",
			SecretKey: "minioadmin",
			Bucket:    s.bucketName,
		},
		Auth: config.AuthConfig{
			Keycloak: config.KeycloakConfig{
				BaseURL:     "http://localhost:9999",
				Realm:       "test",
				ClientID:    "test-client",
				RedirectURI: "http://localhost:18080/auth/callback",
			},
			Session: config.SessionConfig{
				TTL:        3600,
				Secure:     false,
				StateTTL:   300,
				RedisKey:   "mdrive-test",
				CookieName: "session_id",
			},
		},
		CORS: config.CORSConfig{
			Enabled: true,
			AllowedOrigins: []string{
				"http://localhost:3000",
				"https://mdrive.starship.co",
			},
			AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowedHeaders: []string{"Content-Type", "Authorization", "X-Requested-With"},
			ExposedHeaders: []string{"Content-Length", "Content-Type"},
			MaxAge:         86400,
		},
	}

	s.baseURL = fmt.Sprintf("http://%s:%d", s.Config.HTTP.Host, s.Config.HTTP.Port)

	return nil
}

// StartServer starts the HTTP server
func (s *Suite) StartServer(ctx context.Context) error {
	// Create a temporary config file
	tmpDir, err := os.MkdirTemp("", "mdrive-test-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	s.cfgFile = tmpDir + "/config.yaml"

	// Write config to temp file as YAML
	cfgData, err := yaml.Marshal(s.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	err = os.WriteFile(s.cfgFile, cfgData, 0600)
	if err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// Start the actual app with embedded OpenAPI spec
	var appErr error
	s.app, appErr = serve.NewApp(s.cfgFile, s.Config.HTTP.Port, openapispec.Spec)
	if appErr != nil {
		return fmt.Errorf("failed to create app: %w", appErr)
	}

	if err := s.app.Start(); err != nil {
		return fmt.Errorf("failed to start app: %w", err)
	}

	// Wait for server to be ready
	var serverReady bool
	for i := 0; i < 50; i++ {
		resp, err := http.Get(s.baseURL + "/health")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				serverReady = true
				break
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	if !serverReady {
		return fmt.Errorf("server failed to start within 10s")
	}

	return nil
}

// Stop cleans up per-test resources (app, DB, bucket).
// Shared containers are NOT stopped — they're cleaned up by TestMain.
func (s *Suite) Stop(ctx context.Context) error {
	if s.app != nil {
		stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = s.app.Shutdown(stopCtx)
	}

	if s.ValkeyClient != nil {
		s.ValkeyClient.Close()
	}
	if s.EntClient != nil {
		_ = s.EntClient.Close()
	}
	if s.DB != nil {
		_ = s.DB.Close()
	}

	// Drop per-test database
	if s.dbName != "" {
		adminDSN := fmt.Sprintf("host=%s port=%d user=test password=test dbname=postgres sslmode=disable",
			sharedPgHost, sharedPgPort)
		adminDB, err := sql.Open("postgres", adminDSN)
		if err == nil {
			// Terminate connections to the test database before dropping
			_, _ = adminDB.ExecContext(ctx, fmt.Sprintf(
				"SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '%s'", s.dbName))
			_, _ = adminDB.ExecContext(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s", s.dbName))
			_ = adminDB.Close()
		}
	}

	// Remove per-test MinIO bucket
	if s.bucketName != "" {
		client, err := minio.New(s.MinioAddr, &minio.Options{
			Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
			Secure: false,
		})
		if err == nil {
			// Remove all objects in the bucket first
			objectsCh := make(chan minio.ObjectInfo)
			go func() {
				defer close(objectsCh)
				for object := range client.ListObjects(ctx, s.bucketName, minio.ListObjectsOptions{}) {
					if object.Err == nil {
						objectsCh <- object
					}
				}
			}()
			_ = client.RemoveObjects(ctx, s.bucketName, objectsCh, minio.RemoveObjectsOptions{})
			_ = client.RemoveBucket(ctx, s.bucketName)
		}
	}

	// Clean up config file
	if s.cfgFile != "" {
		_ = os.RemoveAll(strings.TrimSuffix(s.cfgFile, "/config.yaml"))
	}

	return nil
}

// BaseURL returns the base URL
func (s *Suite) BaseURL() string {
	return s.baseURL
}

// GetEntClient returns the Ent client
func (s *Suite) GetEntClient() *ent.Client {
	return s.EntClient
}

// GetDB returns the database connection
func (s *Suite) GetDB() *sql.DB {
	return s.DB
}

// GetConfig returns the test config
func (s *Suite) GetConfig() *config.Config {
	return s.Config
}

// createMinioBucket creates a per-test bucket in the shared MinIO container (with retry).
func (s *Suite) createMinioBucket(ctx context.Context) error {
	var lastErr error
	for i := 0; i < 10; i++ {
		client, err := minio.New(s.MinioAddr, &minio.Options{
			Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
			Secure: false,
		})
		if err != nil {
			lastErr = err
			time.Sleep(500 * time.Millisecond)
			continue
		}

		if err := client.MakeBucket(ctx, s.bucketName, minio.MakeBucketOptions{}); err != nil {
			lastErr = err
			time.Sleep(500 * time.Millisecond)
			continue
		}
		return nil
	}
	return fmt.Errorf("failed to create minio bucket after retries: %w", lastErr)
}
