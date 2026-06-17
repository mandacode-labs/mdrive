package app

import (
	"context"
	"database/sql"

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
	cryptopkg "github.com/mandacode-labs/mdrive/internal/crypto"
	"github.com/mandacode-labs/mdrive/internal/logging"
	"github.com/mandacode-labs/mdrive/internal/upload"
	"github.com/valkey-io/valkey-go"
)

// App holds all wired components.
type App struct {
	Cfg *config.Config
	Log *logging.Logger

	NodeSvc      *node.Service
	DriveSvc     *drive.Service
	UserSvc      *user.Service
	UserEx       user.Exister
	UploadReg    upload.Registry

	DB  *sql.DB
	Ent *ent.Client
}

// New wires the infrastructure, core domain services, and the vfs service.
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
	nodeSvc := node.NewService(nodeRepo)

	userRepo := user.NewRepository(entClient)
	userSvc := user.NewService(userRepo)
	userEx := user.NewExisterAdapter(userRepo)

	rootCreator := &rootNodeCreator{svc: nodeSvc}

	var cipher cryptopkg.Cipher = cryptopkg.NoOp{}
	if cfg.Crypto.MasterKey != "" {
		var err error
		cipher, err = cryptopkg.NewAESGCM(cfg.Crypto.MasterKey)
		if err != nil {
			return nil, err
		}
	}

	driveRepo := drive.NewRepository(entClient, cipher)
	driveSvc := drive.NewService(driveRepo, userEx, rootCreator)

	uploadReg, err := newUploadRegistry(ctx, cfg.Valkey)
	if err != nil {
		return nil, err
	}

	return &App{
		Cfg:       cfg,
		Log:       log,
		NodeSvc:   nodeSvc,
		DriveSvc:  driveSvc,
		UserSvc:   userSvc,
		UserEx:    userEx,
		UploadReg: uploadReg,
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

func newUploadRegistry(ctx context.Context, cfg config.ValkeyConfig) (upload.Registry, error) {
	if len(cfg.Addrs) == 0 || cfg.Addrs[0] == "" {
		return upload.NewMemoryRegistry(), nil
	}
	client, err := valkey.NewClient(valkey.ClientOption{
		InitAddress: cfg.Addrs,
		Password:    cfg.Password,
		SelectDB:    cfg.DB,
		TLSConfig:   nil, // TODO: set when cfg.TLS is true
	})
	if err != nil {
		return nil, err
	}
	if err := client.Do(ctx, client.B().Ping().Build()).Error(); err != nil {
		return nil, err
	}
	return upload.NewValkeyRegistry(client), nil
}
