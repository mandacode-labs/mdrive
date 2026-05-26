// Package serve implements the serve command
package serve

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/XSAM/otelsql"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/starfrag-lab/retrowin-go/pkg/api"
	httpSwagger "github.com/swaggo/http-swagger"
	"github.com/valkey-io/valkey-go"

	"github.com/starfrag-lab/retrowin-go/ent"
	corefs "github.com/starfrag-lab/retrowin-go/internal/application/fs"
	"github.com/starfrag-lab/retrowin-go/internal/application/storage"
	"github.com/starfrag-lab/retrowin-go/internal/auth"
	"github.com/starfrag-lab/retrowin-go/internal/config"
	"github.com/starfrag-lab/retrowin-go/internal/core/dentry"
	"github.com/starfrag-lab/retrowin-go/internal/core/inode"
	inoderepo "github.com/starfrag-lab/retrowin-go/internal/core/inode/repository"
	"github.com/starfrag-lab/retrowin-go/internal/core/object"
	objectrepo "github.com/starfrag-lab/retrowin-go/internal/core/object/repository"
	s3storage "github.com/starfrag-lab/retrowin-go/internal/core/object/s3"
	coreuser "github.com/starfrag-lab/retrowin-go/internal/core/user"
	"github.com/starfrag-lab/retrowin-go/internal/handler"
	"github.com/starfrag-lab/retrowin-go/internal/middleware"
	"github.com/starfrag-lab/retrowin-go/internal/service/sysinit"
	"github.com/starfrag-lab/retrowin-go/internal/session"
	sessionRepo "github.com/starfrag-lab/retrowin-go/internal/session/repository"
	"github.com/starfrag-lab/retrowin-go/internal/system"
	systemrepo "github.com/starfrag-lab/retrowin-go/internal/system/repository"
	"github.com/starfrag-lab/retrowin-go/internal/telemetry"
	"github.com/starfrag-lab/retrowin-go/internal/user"
	userrepo "github.com/starfrag-lab/retrowin-go/internal/user/repository"
)

// ProvideConfig provides the config from file.
func ProvideConfig(cfgFile string, port int) (*config.Config, error) {
	var cfg *config.Config
	var err error

	if cfgFile != "" {
		cfg, err = config.LoadFromPath(cfgFile)
	} else {
		cfg, err = config.Load("config.yaml")
	}
	if err != nil {
		return nil, err
	}

	// Override port if specified
	if port != 8080 {
		cfg.HTTP.Port = port
	}

	return cfg, nil
}

// ProvideLogger creates a new zap logger.
func ProvideLogger() *zap.Logger {
	logger, _ := zap.NewProduction()
	return logger
}

// NewEntClient creates a new Ent client.
func NewEntClient(lc fx.Lifecycle, cfg *config.Config, logger *zap.Logger) (*ent.Client, error) {
	// Open database connection with OTel instrumentation
	db, err := otelsql.Open("postgres", cfg.Database.DSN(),
		otelsql.WithAttributes(semconv.DBSystemPostgreSQL),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	// Create Ent driver
	drv := entsql.OpenDB(dialect.Postgres, db)

	// Create Ent client
	client := ent.NewClient(ent.Driver(drv))

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			// Test connection
			if err := db.PingContext(ctx); err != nil {
				return fmt.Errorf("failed to ping database: %w", err)
			}
			logger.Info("connected to database",
				zap.String("host", cfg.Database.Host),
				zap.String("database", cfg.Database.Name),
			)

			// Auto migrate in development
			if cfg.App.Env == "development" {
				if err := client.Schema.Create(ctx); err != nil {
					logger.Warn("failed to auto-migrate schema", zap.Error(err))
				}
			}

			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("closing database connection")
			if err := client.Close(); err != nil {
				return fmt.Errorf("failed to close ent client: %w", err)
			}
			return db.Close()
		},
	})

	return client, nil
}

// ProvideValkeyClient provides the Valkey client.
func ProvideValkeyClient(cfg *config.Config) (valkey.Client, error) {
	if cfg.Cache.Provider != "redis" && cfg.Cache.Provider != "valkey" {
		return nil, nil
	}

	client, err := newValkeyClient(&cfg.Cache.Valkey)
	if err != nil {
		return nil, fmt.Errorf("failed to create valkey client: %w", err)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Do(ctx, client.B().Ping().Build()).Error(); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to connect to cache: %w", err)
	}
	fmt.Printf("Connected to %s\n", cfg.Cache.Provider)

	return client, nil
}

// ProvideSessionTTL provides session TTL from config.
func ProvideSessionTTL(cfg *config.Config) time.Duration {
	return time.Duration(cfg.Auth.Session.TTL) * time.Second
}

// ProvideAuthUserService provides the auth user service.
func ProvideAuthUserService(userSvc user.UserService) auth.UserService {
	return auth.NewUserService(userSvc)
}

// ProvideStorage provides the storage backend based on config.
func ProvideStorage(cfg *config.Config) (object.Storage, error) {
	return s3storage.New(&cfg.Storage)
}

// ProvideTelemetry provides and registers OTel providers.
func ProvideTelemetry(lc fx.Lifecycle, cfg *config.Config, logger *zap.Logger) (*telemetry.Providers, error) {
	providers, err := telemetry.NewProviders(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize telemetry: %w", err)
	}
	if cfg.Telemetry.Enabled {
		logger.Info("telemetry enabled",
			zap.String("endpoint", cfg.Telemetry.Endpoint),
			zap.String("service", cfg.Telemetry.ServiceName),
		)
	}
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return providers.Shutdown(ctx)
		},
	})
	return providers, nil
}

// ProvideHTTPMux provides the HTTP mux with all routes.
func ProvideHTTPMux(
	ogenServer *api.Server,
	cfg *config.Config,
	openAPIPath string,
) *http.ServeMux {
	mux := http.NewServeMux()

	// Serve OpenAPI spec and Swagger UI (register before catch-all)
	mux.HandleFunc("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		// Use configured path from CLI flag
		content, err := os.ReadFile(openAPIPath)
		if err != nil {
			http.Error(w, fmt.Sprintf("OpenAPI spec not found: %s", openAPIPath), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(content)
	})
	mux.HandleFunc("/swagger", httpSwagger.Handler(
		httpSwagger.URL("/openapi.json"),
	))

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"healthy"}`))
	})

	// API routes (catch-all - must be last)
	mux.Handle("/", ogenServer)

	return mux
}

// ProvideHTTPHandler provides the HTTP handler with middleware chain.
func ProvideHTTPHandler(mux *http.ServeMux, callbackCfg *middleware.CallbackConfig, cfg *config.Config) http.Handler {
	var handler http.Handler = mux
	handler = middleware.CallbackMiddleware(callbackCfg)(handler)
	handler = middleware.CORSMiddleware(cfg)(handler)
	handler = middleware.RecoveryMiddleware()(handler)
	if cfg.Telemetry.Enabled {
		handler = otelhttp.NewHandler(handler, "http-server")
	}
	return handler
}

// ProvideHTTPServer provides the HTTP server.
func ProvideHTTPServer(
	handler http.Handler,
	cfg *config.Config,
	lc fx.Lifecycle,
) *http.Server {
	addr := fmt.Sprintf("%s:%d", cfg.HTTP.Host, cfg.HTTP.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			ln, err := net.Listen("tcp", addr)
			if err != nil {
				return fmt.Errorf("failed to listen on %s: %w", addr, err)
			}

			fmt.Printf("Starting server on %s\n", addr)

			go func() {
				if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
					fmt.Printf("Server error: %v\n", err)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			fmt.Println("Shutting down server...")
			return srv.Shutdown(ctx)
		},
	})

	return srv
}

// RegisterShutdownHooks registers shutdown signal handlers.
// Taking srv *http.Server as parameter ensures the HTTP server provider is constructed.
func RegisterShutdownHooks(lc fx.Lifecycle, entClient *ent.Client, valkeyCli valkey.Client, srv *http.Server) {
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			if err := entClient.Close(); err != nil {
				return fmt.Errorf("database close error: %w", err)
			}
			if valkeyCli != nil {
				valkeyCli.Close()
			}
			fmt.Println("Server stopped")
			return nil
		},
	})
}

// WaitForShutdown waits for shutdown signals.
func WaitForShutdown(lc fx.Lifecycle) {
	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, syscall.SIGINT, syscall.SIGTERM)

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() {
				sig := <-shutdownCh
				fmt.Printf("\nReceived signal %v, shutting down...\n", sig)
			}()
			return nil
		},
	})
}

// ProvideOgenServer provides the ogen server.
func ProvideOgenServer(
	h *handler.Handler,
	sessionSvc session.SessionService,
	providers *telemetry.Providers,
) (*api.Server, error) {
	securityHandler := handler.NewSecurityHandler(sessionSvc)
	opts := []api.ServerOption{
		api.WithErrorHandler(h.ErrorHandler),
	}
	if _, ok := providers.TracerProvider.(*sdktrace.TracerProvider); ok {
		opts = append(opts, api.WithTracerProvider(providers.TracerProvider))
	}
	if providers.MeterProvider != nil {
		opts = append(opts, api.WithMeterProvider(providers.MeterProvider))
	}
	return api.NewServer(h, securityHandler, opts...)
}

// FxOptions returns the fx options for the application.
func FxOptions(cfgFile string, port int, openAPIPath string) []fx.Option {
	return []fx.Option{
		// Supply CLI args
		fx.Supply(fx.Annotate(cfgFile, fx.ResultTags(`name:"cfgFile"`))),
		fx.Supply(fx.Annotate(port, fx.ResultTags(`name:"port"`))),
		fx.Supply(fx.Annotate(openAPIPath, fx.ResultTags(`name:"openAPIPath"`))),

		// All providers - single fx.Provide call like serengeti
		fx.Provide(
			fx.Annotate(ProvideConfig, fx.ParamTags(`name:"cfgFile"`, `name:"port"`)),
			ProvideLogger,
			ProvideTelemetry,
			NewEntClient,
			ProvideValkeyClient,
			// Repositories
			userrepo.NewRepository,
			inoderepo.NewRepository,
			objectrepo.NewRepository,
			systemrepo.NewSystemUserRepository,
			systemrepo.NewSystemGroupRepository,
			systemrepo.NewRepository,
			NewValkeySessionRepository,
			ProvideSessionTTL,
			// Auth services
			session.NewSessionService,
			ProvideAuthUserService,
			// OIDC service
			ProvideKeycloak,
			ProvideOIDCService,
			// Domain services
			user.NewService,
			inode.NewService,
			object.NewService,
			coreuser.NewService,      // core/user for UID resolution
			coreuser.NewGroupService, // core/user for group management
			system.NewService,        // system management
			sysinit.NewService,       // system initialization
			// Application services
			dentry.NewService,
			corefs.NewService,
			storage.NewService,
			// Storage
			ProvideStorage,
			// HTTP layer
			handler.NewHandler,
			ProvideOgenServer,
			middleware.ProvideCallbackConfig,
			fx.Annotate(ProvideHTTPMux, fx.ParamTags(``, ``, `name:"openAPIPath"`)),
			ProvideHTTPHandler,
			ProvideHTTPServer,
		),

		fx.Invoke(RegisterShutdownHooks),
		fx.Invoke(WaitForShutdown),
		fx.StartTimeout(30 * time.Second),
		fx.StopTimeout(30 * time.Second),
	}
}

// NewFXApp creates a new fx application.
func NewFXApp(cfgFile string, port int, openAPIPath string) *fx.App {
	return fx.New(FxOptions(cfgFile, port, openAPIPath)...)
}

// newValkeyClient creates a Valkey client based on ValkeyConfig.
func newValkeyClient(cfg *config.ValkeyConfig) (valkey.Client, error) {
	opts := valkey.ClientOption{
		InitAddress: []string{cfg.Addr},
	}
	if cfg.Username != "" {
		opts.Username = cfg.Username
	}
	if cfg.Password != "" {
		opts.Password = cfg.Password
	}
	if cfg.DB > 0 {
		opts.SelectDB = cfg.DB
	}
	if cfg.PoolSize > 0 {
		opts.BlockingPoolSize = cfg.PoolSize
	}

	return valkey.NewClient(opts)
}

// NewValkeySessionRepository provides the Valkey session repository.
func NewValkeySessionRepository(client valkey.Client) session.SessionRepository {
	if client == nil {
		return nil
	}
	return sessionRepo.NewValkeySessionRepository(client, "retrowin:session:")
}

// ProvideKeycloak provides the Keycloak OIDC client.
func ProvideKeycloak(cfg *config.Config) *auth.Keycloak {
	issuerURL := cfg.Auth.Keycloak.BaseURL + "/realms/" + cfg.Auth.Keycloak.Realm
	return auth.NewKeycloak(auth.KeycloakConfig{
		Issuer:       issuerURL,
		ClientID:     cfg.Auth.Keycloak.ClientID,
		ClientSecret: cfg.Auth.Keycloak.ClientSecret,
		RedirectURI:  cfg.Auth.Keycloak.RedirectURI,
	})
}

// ProvideOIDCService provides the OIDC service.
func ProvideOIDCService(
	keycloak *auth.Keycloak,
	sessionSvc session.SessionService,
	userSvc auth.UserService,
	client valkey.Client,
	cfg *config.Config,
) (auth.AuthService, error) {
	stateTTL := time.Duration(cfg.Auth.Session.StateTTL) * time.Second
	return auth.NewService(keycloak, sessionSvc, userSvc, client, cfg.Auth.Session.RedisKey, stateTTL, cfg.Auth.Session.FrontendURL)
}
