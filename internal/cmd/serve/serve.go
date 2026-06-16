// Package serve provides the HTTP server entry point.
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
	"github.com/google/uuid"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"github.com/mandacode-labs/mdrive/ent"
	"github.com/mandacode-labs/mdrive/internal/config"
	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/core/user"
	"github.com/mandacode-labs/mdrive/internal/logging"
)

// App holds the application components and lifecycle.
type App struct {
	cfg  *config.Config
	log  *logging.Logger
	ent  *ent.Client
	http *http.Server
}

// NewApp creates and initializes all components.
func NewApp(cfg *config.Config, version string) (*App, error) {
	log := logging.NewLogger(cfg.App.Env, cfg.App.LogLevel)

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
	log.Info().
		Str("host", cfg.Database.Host).
		Str("database", cfg.Database.Name).
		Msg("connected to database")

	// Run auto-migration (dev convenience; in production use versioned migrations).
	if cfg.App.Env == "development" {
		if err := entClient.Schema.Create(context.Background()); err != nil {
			log.Warn().Err(err).Msg("auto-migration failed")
		}
	}

	// Wire core services.
	nodeRepo := node.NewEntRepository(entClient)
	nodeSvc := node.NewService(nodeRepo)

	userRepo := user.NewRepository(entClient)
	userSvc := user.NewService(userRepo)

	userExister := user.NewExisterAdapter(userRepo)

	// Drive's RootCreator adapter calls into node.Service to create the root dir.
	rootCreator := &nodeRootCreator{svc: nodeSvc}

	driveRepo := drive.NewRepository(entClient)
	driveSvc := drive.NewService(driveRepo, userExister, rootCreator)

	// Suppress unused warnings for now (these will be wired into vfs/handler later).
	_ = nodeSvc
	_ = userSvc
	_ = driveSvc

	// Build a minimal HTTP router with health and version endpoints.
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(version))
	})
	mux.HandleFunc("/_internal/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("pong"))
	})

	srv := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", cfg.HTTP.Host, cfg.HTTP.Port),
		Handler:           mux,
		ReadHeaderTimeout: 30 * time.Second,
	}

	return &App{
		cfg:  cfg,
		log:  log,
		ent:  entClient,
		http: srv,
	}, nil
}

// nodeRootCreator adapts node.Service to drive.RootCreator.
type nodeRootCreator struct {
	svc *node.Service
}

func (n *nodeRootCreator) NewRootDirectory(ctx context.Context) (uuid.UUID, error) {
	root, err := n.svc.CreateDirectory(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return root.ID(), nil
}

// Run starts the server and blocks until shutdown.
func (a *App) Run() error {
	addr := a.http.Addr
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}
	a.log.Info().Str("addr", addr).Msg("starting server")
	go func() {
		if err := a.http.Serve(ln); err != nil && err != http.ErrServerClosed {
			a.log.Error().Err(err).Msg("server error")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	a.log.Info().Msg("received shutdown signal")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return a.Shutdown(shutdownCtx)
}

// Shutdown stops the server and closes the DB connection.
func (a *App) Shutdown(ctx context.Context) error {
	if err := a.http.Shutdown(ctx); err != nil {
		a.log.Error().Err(err).Msg("http shutdown error")
	}
	if err := a.ent.Close(); err != nil {
		a.log.Error().Err(err).Msg("ent close error")
	}
	a.log.Info().Msg("server stopped")
	return nil
}

// Run is a top-level convenience that constructs and runs the app.
func Run(cfg *config.Config, version string) error {
	app, err := NewApp(cfg, version)
	if err != nil {
		return err
	}
	return app.Run()
}
