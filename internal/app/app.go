package app

import (
	"context"
	"database/sql"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/XSAM/otelsql"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"github.com/mandacode-labs/mdrive/ent"
	"github.com/mandacode-labs/mdrive/internal/config"
	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/core/user"
	"github.com/mandacode-labs/mdrive/internal/logging"
)

// App holds all wired components (bare repos, no services).
//
// Service composition (orchestration across domains, permission checks,
// S3 interaction) lives in internal/vfs. This package only wires the
// infrastructure and core repositories.
type App struct {
	Cfg *config.Config
	Log *logging.Logger

	NodeRepo  node.Repository
	DriveRepo drive.Repository
	UserRepo  user.Repository

	DB  *sql.DB
	Ent *ent.Client
}

// New wires the core infrastructure and repositories.
func New(ctx context.Context, cfg *config.Config) (*App, error) {
	log := logging.NewLogger(cfg.App.Env, cfg.App.LogLevel)

	db, err := otelsql.Open("postgres", cfg.Database.DSN(),
		otelsql.WithAttributes(semconv.DBSystemPostgreSQL),
	)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	log.Info().Str("host", cfg.Database.Host).Str("database", cfg.Database.Name).Msg("connected to database")

	drv := entsql.OpenDB(dialect.Postgres, db)
	entClient := ent.NewClient(ent.Driver(drv))

	if cfg.App.Env == "development" {
		if err := entClient.Schema.Create(ctx); err != nil {
			log.Warn().Err(err).Msg("auto-migration failed")
		}
	}

	nodeRepo := node.NewEntRepository(entClient)
	driveRepo := drive.NewRepository(entClient)
	userRepo := user.NewRepository(entClient)

	return &App{
		Cfg:       cfg,
		Log:       log,
		NodeRepo:  nodeRepo,
		DriveRepo: driveRepo,
		UserRepo:  userRepo,
		DB:        db,
		Ent:       entClient,
	}, nil
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
