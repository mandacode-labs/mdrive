package serve

import (
	"context"
	"database/sql"
	"errors"
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
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"github.com/mandacode-labs/retrowin-go/ent"
	"github.com/mandacode-labs/retrowin-go/internal/application/storage"
	corefs "github.com/mandacode-labs/retrowin-go/internal/application/vfs"
	"github.com/mandacode-labs/retrowin-go/internal/config"
	"github.com/mandacode-labs/retrowin-go/internal/core/inode"
	inoderepo "github.com/mandacode-labs/retrowin-go/internal/core/inode/repository"
	"github.com/mandacode-labs/retrowin-go/internal/core/object"
	objectrepo "github.com/mandacode-labs/retrowin-go/internal/core/object/repository"
	coreuser "github.com/mandacode-labs/retrowin-go/internal/core/user"
	"github.com/mandacode-labs/retrowin-go/internal/handler"
	"github.com/mandacode-labs/retrowin-go/internal/logging"
	"github.com/mandacode-labs/retrowin-go/internal/middleware"
	"github.com/mandacode-labs/retrowin-go/internal/service/sysinit"
	"github.com/mandacode-labs/retrowin-go/internal/session"
	"github.com/mandacode-labs/retrowin-go/internal/system"
	systemrepo "github.com/mandacode-labs/retrowin-go/internal/system/repository"
	"github.com/mandacode-labs/retrowin-go/internal/telemetry"
	"github.com/mandacode-labs/retrowin-go/internal/user"
	userrepo "github.com/mandacode-labs/retrowin-go/internal/user/repository"
	"github.com/mandacode-labs/retrowin-go/pkg/api"
	"github.com/valkey-io/valkey-go"
)

// App holds all application components and manages their lifecycle.
type App struct {
	cfg        *config.Config
	logger     *logging.Logger
	entClient  *ent.Client
	db         *sql.DB
	valkeyCli  valkey.Client
	httpServer *http.Server
	telemetry  *telemetry.Providers
}

// NewApp creates all application components with explicit dependency injection.
func NewApp(cfgFile string, port int, openAPISpec []byte) (*App, error) {
	cfg, err := ProvideConfig(cfgFile, port)
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	log := ProvideLogger(cfg)

	telemetryProviders, err := telemetry.NewProviders(cfg)
	if err != nil {
		return nil, fmt.Errorf("telemetry: %w", err)
	}

	db, err := otelsql.Open("postgres", cfg.Database.DSN(),
		otelsql.WithAttributes(semconv.DBSystemPostgreSQL),
	)
	if err != nil {
		return nil, fmt.Errorf("database: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	drv := entsql.OpenDB(dialect.Postgres, db)
	entClient := ent.NewClient(ent.Driver(drv))

	if err := db.PingContext(context.Background()); err != nil {
		return nil, fmt.Errorf("database ping: %w", err)
	}
	log.Info().Str("host", cfg.Database.Host).Str("database", cfg.Database.Name).Msg("connected to database")
	if cfg.App.Env == "development" {
		if err := entClient.Schema.Create(context.Background()); err != nil {
			log.Warn().Err(err).Msg("failed to auto-migrate schema")
		}
	}

	valkeyCli, err := ProvideValkeyClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("valkey: %w", err)
	}

	objectStorage, err := ProvideStorage(cfg)
	if err != nil {
		return nil, fmt.Errorf("storage: %w", err)
	}
	userRepo := userrepo.NewRepository(entClient)
	inodeRepo := inoderepo.NewRepository(entClient)
	objectRepo := objectrepo.NewRepository(entClient)
	sysUserRepo := systemrepo.NewSystemUserRepository(entClient)
	sysGroupRepo := systemrepo.NewSystemGroupRepository(entClient)
	sysRepo := systemrepo.NewRepository(entClient)
	sessionTTL := ProvideSessionTTL(cfg)
	sessionRepo := NewValkeySessionRepository(valkeyCli)

	extUserSvc := user.NewService(userRepo)
	inodeSvc := inode.NewService(inodeRepo)
	objectSvc := object.NewService(objectRepo, objectStorage)
	coreUserSvc := coreuser.NewService(sysUserRepo, sysGroupRepo)
	coreGroupSvc := coreuser.NewGroupService(sysGroupRepo)
	systemSvc := system.NewService(sysRepo, inodeSvc, objectSvc, coreUserSvc, coreGroupSvc)

	dirLock := corefs.NewLocker()
	fsSvc := corefs.NewService(entClient, inodeSvc, objectSvc, objectStorage, coreUserSvc, dirLock)
	storageSvc := storage.NewService(fsSvc, objectSvc)
	initSvc := sysinit.NewService(systemSvc, coreUserSvc, fsSvc, inodeSvc)

	sessionSvc := session.NewSessionService(sessionRepo, sessionTTL)
	keycloak := ProvideKeycloak(cfg)
	authUserSvc := ProvideAuthUserService(extUserSvc)
	authSvc, err := ProvideOIDCService(keycloak, sessionSvc, authUserSvc, valkeyCli, cfg)
	if err != nil {
		return nil, fmt.Errorf("auth: %w", err)
	}

	h := handler.NewHandler(authSvc, extUserSvc, coreUserSvc, coreGroupSvc, systemSvc, fsSvc, inodeSvc, storageSvc, initSvc)
	securityHandler := handler.NewSecurityHandler(sessionSvc)
	ogenOpts := []api.ServerOption{
		api.WithErrorHandler(h.ErrorHandler),
	}
	if telemetryProviders.TracerProvider != nil {
		ogenOpts = append(ogenOpts, api.WithTracerProvider(telemetryProviders.TracerProvider))
	}
	if telemetryProviders.MeterProvider != nil {
		ogenOpts = append(ogenOpts, api.WithMeterProvider(telemetryProviders.MeterProvider))
	}
	ogenServer, err := api.NewServer(h, securityHandler, ogenOpts...)
	if err != nil {
		return nil, fmt.Errorf("ogen server: %w", err)
	}

	callbackCfg := middleware.ProvideCallbackConfig(cfg)
	mux := ProvideHTTPMux(ogenServer, cfg, openAPISpec)
	httpHandler := ProvideHTTPHandler(mux, callbackCfg, cfg, log)
	srv := ProvideHTTPServer(httpHandler, cfg, log)

	return &App{
		cfg:        cfg,
		logger:     log,
		entClient:  entClient,
		db:         db,
		valkeyCli:  valkeyCli,
		httpServer: srv,
		telemetry:  telemetryProviders,
	}, nil
}

// Start starts the HTTP server in the background.
func (a *App) Start() error {
	addr := fmt.Sprintf("%s:%d", a.cfg.HTTP.Host, a.cfg.HTTP.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}
	a.logger.Info().Str("addr", addr).Msg("starting server")
	go func() {
		if err := a.httpServer.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.logger.Error().Err(err).Msg("server error")
		}
	}()
	return nil
}

// Wait blocks until a shutdown signal is received.
func (a *App) Wait() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	a.logger.Info().Str("signal", sig.String()).Msg("received shutdown signal")
}

// Shutdown performs a graceful shutdown with the given timeout.
func (a *App) Shutdown(ctx context.Context) error {
	if err := a.httpServer.Shutdown(ctx); err != nil {
		a.logger.Error().Err(err).Msg("server shutdown error")
	}
	if a.valkeyCli != nil {
		a.valkeyCli.Close()
	}
	if err := a.entClient.Close(); err != nil {
		a.logger.Error().Err(err).Msg("database close error")
	}
	if err := a.db.Close(); err != nil {
		a.logger.Error().Err(err).Msg("database connection close error")
	}
	if err := a.telemetry.Shutdown(ctx); err != nil {
		return fmt.Errorf("telemetry shutdown: %w", err)
	}
	a.logger.Info().Msg("server stopped")
	return nil
}

// Run starts the server and blocks until shutdown. Convenience for production use.
func (a *App) Run() {
	if err := a.Start(); err != nil {
		a.logger.Fatal().Err(err).Msg("failed to start server")
	}
	a.Wait()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = a.Shutdown(shutdownCtx)
}
