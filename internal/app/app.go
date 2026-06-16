// Package app constructs the application: wires core domains, storage,
// permissions, and the various runtime components (apiserver, gc job).
//
// `app` is the composition root — no domain code should import it.
package app

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/XSAM/otelsql"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/ent"
	"github.com/mandacode-labs/mdrive/internal/config"
	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/core/user"
	"github.com/mandacode-labs/mdrive/internal/logging"
	"github.com/mandacode-labs/mdrive/internal/storage"
	"github.com/mandacode-labs/mdrive/internal/storage/s3"
)

// App holds all wired components.
type App struct {
	Cfg *config.Config
	Log *logging.Logger

	// Core services
	NodeSvc *node.Service
	UserSvc *user.Service
	DriveSvc *drive.Service

	// Storage (S3 client factory)
	Storage storage.Storage

	// Database client (for cleanup)
	DB   *sql.DB
	Ent  *ent.Client
}

// New constructs the application with all dependencies wired.
// This is the single composition root used by both api-server and gc.
func New(ctx context.Context, cfg *config.Config) (*App, error) {
	log := logging.NewLogger(cfg.App.Env, cfg.App.LogLevel)

	// Database
	db, err := otelsql.Open("postgres", cfg.Database.DSN(),
		otelsql.WithAttributes(semconv.DBSystemPostgreSQL),
	)
	if err != nil {
		return nil, fmt.Errorf("database open: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("database ping: %w", err)
	}
	log.Info().
		Str("host", cfg.Database.Host).
		Str("database", cfg.Database.Name).
		Msg("connected to database")

	drv := entsql.OpenDB(dialect.Postgres, db)
	entClient := ent.NewClient(ent.Driver(drv))

	// Auto-migration in dev (in production use versioned migrations).
	if cfg.App.Env == "development" {
		if err := entClient.Schema.Create(ctx); err != nil {
			log.Warn().Err(err).Msg("auto-migration failed")
		}
	}

	// Core services
	nodeRepo := node.NewEntRepository(entClient)
	nodeSvc := node.NewService(nodeRepo)

	userRepo := user.NewRepository(entClient)
	userSvc := user.NewService(userRepo)
	userExister := user.NewExisterAdapter(userRepo)

	// RootCreator adapter: drive.Service needs a way to create a root dir node.
	rootCreator := &rootNodeCreator{svc: nodeSvc}

	driveRepo := drive.NewRepository(entClient)
	driveSvc := drive.NewService(driveRepo, userExister, rootCreator)

	// Storage factory: a single S3 client is created per drive (because credentials
	// are per-drive). For now we just wire a placeholder; the real storage client
	// is created on demand from a drive's storage config.
	store := newLazyStorage()

	return &App{
		Cfg:       cfg,
		Log:       log,
		NodeSvc:   nodeSvc,
		UserSvc:   userSvc,
		DriveSvc:  driveSvc,
		Storage:   store,
		DB:        db,
		Ent:       entClient,
	}, nil
}

// rootNodeCreator adapts node.Service to drive.RootCreator.
type rootNodeCreator struct {
	svc *node.Service
}

func (n *rootNodeCreator) NewRootDirectory(ctx context.Context) (uuid.UUID, error) {
	root, err := n.svc.CreateDirectory(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return root.ID(), nil
}

// Close releases the database connection.
func (a *App) Close() error {
	if a.Ent != nil {
		_ = a.Ent.Close()
	}
	if a.DB != nil {
		return a.DB.Close()
	}
	return nil
}

// lazyStorage returns a Storage that creates a fresh S3 client on each call,
// using the current drive's storage config.
//
// This is a placeholder until the application/vfs (or vfs) service is wired in.
type lazyStorage struct{}

func newLazyStorage() *lazyStorage { return &lazyStorage{} }

// GetObject is unimplemented at the app layer; callers should resolve the drive
// and use a per-drive S3 client.
func (l *lazyStorage) GetObject(_ context.Context, _, _ string) ([]byte, error) {
	return nil, fmt.Errorf("app.lazyStorage: use drive-specific S3 client")
}

// PutObject is unimplemented.
func (l *lazyStorage) PutObject(_ context.Context, _, _ string, _ io.Reader, _ int64) error {
	return fmt.Errorf("app.lazyStorage: use drive-specific S3 client")
}

// DeleteObject is unimplemented.
func (l *lazyStorage) DeleteObject(_ context.Context, _, _ string) error {
	return fmt.Errorf("app.lazyStorage: use drive-specific S3 client")
}

// DeleteObjects is unimplemented.
func (l *lazyStorage) DeleteObjects(_ context.Context, _ string, _ []string) error {
	return fmt.Errorf("app.lazyStorage: use drive-specific S3 client")
}

// ObjectExists is unimplemented.
func (l *lazyStorage) ObjectExists(_ context.Context, _, _ string) (bool, error) {
	return false, fmt.Errorf("app.lazyStorage: use drive-specific S3 client")
}

// GetObjectSize is unimplemented.
func (l *lazyStorage) GetObjectSize(_ context.Context, _, _ string) (int64, error) {
	return 0, fmt.Errorf("app.lazyStorage: use drive-specific S3 client")
}

// GetObjectChecksum is unimplemented.
func (l *lazyStorage) GetObjectChecksum(_ context.Context, _, _ string) (string, error) {
	return "", fmt.Errorf("app.lazyStorage: use drive-specific S3 client")
}

// GetPresignedUploadURL is unimplemented.
func (l *lazyStorage) GetPresignedUploadURL(_ context.Context, _, _, _ string, _ int64, _ string, _ time.Duration) (string, error) {
	return "", fmt.Errorf("app.lazyStorage: use drive-specific S3 client")
}

// GetPresignedDownloadURL is unimplemented.
func (l *lazyStorage) GetPresignedDownloadURL(_ context.Context, _, _ string, _ time.Duration) (string, error) {
	return "", fmt.Errorf("app.lazyStorage: use drive-specific S3 client")
}

// Compile-time interface check.
var _ storage.Storage = (*lazyStorage)(nil)

// Ensure storage/s3 is referenced so the import isn't removed by gofmt
// (we keep it here so future wiring can use it).
var _ = s3.NewClient
