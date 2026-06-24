package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/XSAM/otelsql"
	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"github.com/mandacode-labs/mdrive/ent"
	"github.com/mandacode-labs/mdrive/internal/app/gc"
	"github.com/mandacode-labs/mdrive/internal/auth"
	"github.com/mandacode-labs/mdrive/internal/auth/session"
	"github.com/mandacode-labs/mdrive/internal/config"
	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/core/user"
	cryptopkg "github.com/mandacode-labs/mdrive/internal/crypto"
	"github.com/mandacode-labs/mdrive/internal/permission"
	"github.com/mandacode-labs/mdrive/internal/upload"
	"github.com/mandacode-labs/mdrive/internal/upload/s3"
	"github.com/mandacode-labs/mdrive/internal/vfs"
)

// App holds all wired components. The fields are split into the
// three consumers they serve: HTTP transport (NodeSvc, DriveSvc,
// UserSvc, UploadReg, UploadSvc, VFS, SessionStore, Auth,
// Security, Perm, Garbage), background jobs (everything HTTP
// needs plus the same), and lifecycle (DB, Ent, Cfg, Log).
type App struct {
	Cfg *config.Config
	Log *slog.Logger

	NodeSvc      *node.Service
	DriveSvc     *drive.Service
	UserSvc      *user.Service
	UserEx       user.Exister
	UploadReg    upload.Registry
	UploadSvc    *upload.Service
	VFS          *vfs.Service
	SessionStore session.Store
	Garbage      *gc.GarbageRecorder
	Auth         *auth.Service
	Security     *auth.SecurityHandler
	Perm         permission.Checker

	DB  *sql.DB
	Ent *ent.Client
}

// New wires the application by composing a sequence of small
// builders. Each builder is 10-20 lines, single-purpose, and
// fail-fast on its own concern. New itself only does config
// validation and the composition.
func New(ctx context.Context, cfg *config.Config) (*App, error) {
	if err := cfg.Validate(cfg.App.Env); err != nil {
		return nil, err
	}

	log := newLogger(cfg.App.Env, cfg.App.LogLevel)

	db, entClient, err := newInfra(ctx, cfg, log)
	if err != nil {
		return nil, err
	}

	cipher, err := newCrypto(ctx, cfg, log)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	repos := newRepositories(entClient, cipher)

	vClient, uploadReg, err := newValkeyClient(ctx, cfg.Valkey)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	s3Client, err := newS3Client(ctx, cfg.Storage)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	permClient, err := newPerm(ctx, cfg, log)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	store, authenticator, sec, err := newAuth(ctx, cfg, vClient)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	return &App{
		Cfg:          cfg,
		Log:          log,
		NodeSvc:      repos.NodeSvc,
		DriveSvc:     repos.DriveSvc,
		UserSvc:      repos.UserSvc,
		UserEx:       repos.UserEx,
		UploadReg:    uploadReg,
		UploadSvc:    newUpload(repos, s3Client, uploadReg, entClient),
		VFS:          newVFS(repos, entClient, log),
		SessionStore: store,
		Garbage:      gc.NewGarbageRecorder(entClient),
		Auth:         authenticator,
		Security:     sec,
		Perm:         permClient,
		DB:           db,
		Ent:          entClient,
	}, nil
}

// newLogger returns a *slog.Logger configured for the given
// environment and level. In non-production the output goes to
// stderr with a text handler for human-readable logs; in
// production the output is JSON suitable for ingestion by an
// aggregator.
func newLogger(env, level string) *slog.Logger {
	lvl := parseLogLevel(level)
	opts := &slog.HandlerOptions{Level: lvl}
	var h slog.Handler
	if env == "production" {
		h = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		h = slog.NewTextHandler(os.Stderr, opts)
	}
	return slog.New(h).With("env", env)
}

func parseLogLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// newInfra opens the database and ent client. In development
// the ent schema is auto-created; in production the caller is
// expected to run migrations separately.
func newInfra(ctx context.Context, cfg *config.Config, log *slog.Logger) (*sql.DB, *ent.Client, error) {
	db, err := otelsql.Open("postgres", cfg.Database.DSN(),
		otelsql.WithAttributes(semconv.DBSystemPostgreSQL),
	)
	if err != nil {
		return nil, nil, err
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, nil, err
	}
	log.Info("connected to database", "host", cfg.Database.Host, "database", cfg.Database.Name)

	drv := entsql.OpenDB(dialect.Postgres, db)
	entClient := ent.NewClient(ent.Driver(drv))

	if cfg.App.Env == "development" {
		if err := entClient.Schema.Create(ctx); err != nil {
			log.Warn("auto-migration failed", "err", err)
		}
	}

	return db, entClient, nil
}

// newCrypto returns the at-rest cipher. In production the
// master key is required; in development the NoOp cipher is
// used with a loud startup log so the operator sees the data
// is unprotected.
func newCrypto(_ context.Context, cfg *config.Config, log *slog.Logger) (cryptopkg.Cipher, error) {
	if cfg.Crypto.MasterKey == "" {
		if cfg.App.Env == "production" {
			return nil, fmt.Errorf("crypto: master key required in production (set CRYPTO_MASTER_KEY or crypto.master_key)")
		}
		log.Warn("crypto: master key not configured; using NoOp cipher (drive storage secrets will be stored in plaintext)")
		return cryptopkg.NoOp{}, nil
	}
	cipher, err := cryptopkg.NewAESGCM(cfg.Crypto.MasterKey)
	if err != nil {
		return nil, fmt.Errorf("crypto: initialize cipher: %w", err)
	}
	return cipher, nil
}

// repositories groups the three core services so the
// construction step is one assignment in New.
type repositories struct {
	NodeSvc  *node.Service
	DriveSvc *drive.Service
	UserSvc  *user.Service
	UserEx   user.Exister
}

// newRepositories builds the three core domain services. The
// drive service needs a root creator that wraps node.Service;
// the user service exposes an Exister adapter for the drive
// service to use.
func newRepositories(entClient *ent.Client, cipher cryptopkg.Cipher) repositories {
	nodeRepo := node.NewEntRepository(entClient)
	nodeSvc := node.NewService(nodeRepo)

	userRepo := user.NewRepository(entClient)
	userSvc := user.NewService(userRepo)
	userEx := user.NewExisterAdapter(userRepo)

	rootCreator := &rootNodeCreator{svc: nodeSvc}
	driveRepo := drive.NewRepository(entClient, cipher)
	driveSvc := drive.NewService(driveRepo, userEx, rootCreator, nil)

	return repositories{
		NodeSvc:  nodeSvc,
		DriveSvc: driveSvc,
		UserSvc:  userSvc,
		UserEx:   userEx,
	}
}

// newPerm returns the OpenFGA permission checker. nil in dev
// (permission.Require is permissive), required in production.
func newPerm(ctx context.Context, cfg *config.Config, log *slog.Logger) (permission.Checker, error) {
	if cfg.OpenFGA.APIURL == "" {
		if cfg.App.Env == "production" {
			return nil, fmt.Errorf("openfga: APIURL required in production (set OPENFGA_APIURL or openfga.api_url)")
		}
		log.Warn("openfga: APIURL not configured; permission checks disabled (AnonSecurity will be used by the HTTP layer)")
		return nil, nil
	}
	checker, err := permission.NewOpenFGAChecker(ctx, permission.Config{
		AuthMode:             permission.AuthMode(cfg.OpenFGA.AuthMode),
		APIURL:               cfg.OpenFGA.APIURL,
		StoreID:              cfg.OpenFGA.StoreID,
		AuthorizationModelID: cfg.OpenFGA.AuthorizationModelID,
		APIToken:             cfg.OpenFGA.APIToken,
		ClientID:             cfg.OpenFGA.ClientID,
		ClientSecret:         cfg.OpenFGA.ClientSecret,
		TokenIssuer:          cfg.OpenFGA.TokenIssuer,
		Audience:             cfg.OpenFGA.Audience,
	})
	if err != nil {
		return nil, fmt.Errorf("openfga: initialize: %w", err)
	}
	return checker, nil
}

// newAuth wires OIDC + session store. In dev (no auth config)
// the store is in-memory and the security handler is nil; the
// HTTP layer will fall back to AnonSecurity.
func newAuth(ctx context.Context, cfg *config.Config, vClient valkey.Client) (session.Store, *auth.Service, *auth.SecurityHandler, error) {
	var store session.Store = session.NewMemoryStore()
	if cfg.Auth.Issuer == "" || cfg.Auth.ClientID == "" {
		return store, nil, nil, nil
	}
	if vClient != nil {
		store = session.NewValkeyStore(vClient)
	}
	authenticator, err := auth.NewService(ctx, auth.Config{
		Issuer:       cfg.Auth.Issuer,
		ClientID:     cfg.Auth.ClientID,
		SessionStore: store,
		SessionTTL:   cfg.Auth.SessionTTL,
		FrontendURL:  cfg.Auth.FrontendURL,
	})
	if err != nil {
		return nil, nil, nil, err
	}
	sec := auth.NewSecurityHandler(authenticator, cfg.HTTP.Cookie.Name)
	return store, authenticator, sec, nil
}

// newVFS builds the inode-tree manager. vfs has no S3 or HTTP
// dependency: it manages paths, links, and the tree; it
// notifies Garbage when object nodes go away.
func newVFS(repos repositories, entClient *ent.Client, log *slog.Logger) *vfs.Service {
	return vfs.NewService(vfs.ServiceConfig{
		Node:    repos.NodeSvc,
		Drive:   repos.DriveSvc,
		Garbage: gc.NewGarbageRecorder(entClient),
		Logger:  log,
	})
}

// newUpload builds the S3 lifecycle service. Permission is the
// handler's responsibility; the service is the pure
// presign/complete/delete flow.
func newUpload(repos repositories, store *s3.Client, reg upload.Registry, _ *ent.Client) *upload.Service {
	return upload.NewService(upload.Config{
		Reg:   reg,
		Drive: repos.DriveSvc,
		Nodes: repos.NodeSvc,
		Store: store,
		Path:  nil, // set below: depends on vfs which depends on Garbage
	})
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

// newS3Client builds an upload/s3 client. The same instance is
// shared by upload.Service (presign + delete) and the gc
// tombstone cleaner.
func newS3Client(ctx context.Context, cfg config.StorageConfig) (*s3.Client, error) {
	endpoint := (*string)(nil)
	if cfg.Endpoint != "" {
		endpoint = &cfg.Endpoint
	}
	return s3.NewClient(ctx, s3.Config{
		Region:       cfg.Region,
		Endpoint:     endpoint,
		AccessKey:    cfg.AccessKey,
		SecretKey:    cfg.SecretKey,
		UsePathStyle: cfg.UsePathStyle,
	})
}

func newValkeyClient(ctx context.Context, cfg config.ValkeyConfig) (valkey.Client, upload.Registry, error) {
	if len(cfg.Addrs) == 0 || cfg.Addrs[0] == "" {
		return nil, upload.NewMemoryRegistry(), nil
	}
	client, err := valkey.NewClient(valkey.ClientOption{
		InitAddress: cfg.Addrs,
		Password:    cfg.Password,
		SelectDB:    cfg.DB,
	})
	if err != nil {
		return nil, nil, err
	}
	if err := client.Do(ctx, client.B().Ping().Build()).Error(); err != nil {
		return nil, nil, err
	}
	return client, upload.NewValkeyRegistry(client), nil
}
