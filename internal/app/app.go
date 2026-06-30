package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/XSAM/otelsql"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/valkey-io/valkey-go"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"github.com/mandacode-labs/mdrive/ent"
	"github.com/mandacode-labs/mdrive/internal/app/gc"
	"github.com/mandacode-labs/mdrive/internal/auth"
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

// App holds all wired components. Fields are grouped loosely
// into HTTP-serving services (NodeSvc, DriveSvc, ..., Auth),
// background-job services (same, plus Garbage), and lifecycle
// (DB, Ent, Config, Log).
type App struct {
	Config *config.Config
	Log    *slog.Logger

	NodeSvc      *node.Service
	DriveSvc     *drive.Service
	UserSvc      *user.Service
	OwnerChecker drive.OwnerChecker
	UploadToken  upload.TokenRegistry
	UploadSvc    *upload.Service
	VFS          *vfs.Service
	Garbage      *gc.GarbageRecorder
	Auth         *auth.Service
	Security     *auth.SecurityHandler
	Authorizer   permission.Authorizer

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
	cfg.MigrateDeprecatedAuth()

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

	garbage := gc.NewGarbageRecorder(entClient)

	uploadReg, err := newUploadRegistry(ctx, cfg.Valkey, log)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	s3Client, err := newS3Client(ctx, cfg.Storage, log)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	permClient, err := newPerm(ctx, cfg, log)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	authenticator, sec, err := newAuth(ctx, cfg, log, repos.UserSvc)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	return &App{
		Config:       cfg,
		Log:          log,
		NodeSvc:      repos.NodeSvc,
		DriveSvc:     repos.DriveSvc,
		UserSvc:      repos.UserSvc,
		OwnerChecker: repos.OwnerChecker,
		UploadToken:  uploadReg,
		UploadSvc:    newUpload(repos, s3Client, uploadReg),
		VFS:          newVFS(repos, garbage, log),
		Garbage:      garbage,
		Auth:         authenticator,
		Security:     sec,
		Authorizer:   permClient,
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
	NodeSvc      *node.Service
	DriveSvc     *drive.Service
	UserSvc      *user.Service
	OwnerChecker drive.OwnerChecker
}

// newRepositories builds the three core domain services. The
// drive service needs a root creator that wraps node.Service;
// drive verifies owners via the user.Repository directly.
func newRepositories(entClient *ent.Client, cipher cryptopkg.Cipher) repositories {
	nodeRepo := node.NewRepository(entClient)
	nodeSvc := node.NewService(nodeRepo)

	userRepo := user.NewRepository(entClient)
	userSvc := user.NewService(userRepo)

	rootCreator := &rootNodeCreator{svc: nodeSvc}
	driveRepo := drive.NewRepository(entClient, cipher)
	driveSvc := drive.NewService(driveRepo, userRepo, rootCreator)

	return repositories{
		NodeSvc:      nodeSvc,
		DriveSvc:     driveSvc,
		UserSvc:      userSvc,
		OwnerChecker: userRepo,
	}
}

// newPerm returns the OpenFGA Authorizer. In production a real
// FGAChecker is required; in development without an OpenFGA
// configured, an explicit NopAuthorizer is returned so callers
// can wire the same Authorizer interface without a hidden
// fail-open default.
func newPerm(ctx context.Context, cfg *config.Config, log *slog.Logger) (permission.Authorizer, error) {
	if cfg.OpenFGA.APIURL == "" {
		if cfg.App.Env == "production" {
			return nil, fmt.Errorf("openfga: APIURL required in production (set OPENFGA_APIURL or openfga.api_url)")
		}
		log.Warn("openfga: APIURL not configured; using NopAuthorizer (development only)")
		return permission.NopAuthorizer{}, nil
	}
	checker, err := permission.NewFGAChecker(ctx, permission.Config{
		AuthMode:             permission.AuthMode(cfg.OpenFGA.AuthMode),
		APIURL:               cfg.OpenFGA.APIURL,
		StoreID:              cfg.OpenFGA.StoreID,
		AuthorizationModelID: cfg.OpenFGA.AuthorizationModelID,
		APIToken:             cfg.OpenFGA.APIToken,
		ClientID:             cfg.OpenFGA.ClientID,
		ClientSecret:         cfg.OpenFGA.ClientSecret,
		TokenIssuer:          cfg.OpenFGA.TokenIssuer,
		Audience:             cfg.OpenFGA.Audience,
		Scopes:               cfg.OpenFGA.Scopes,
	})
	if err != nil {
		return nil, fmt.Errorf("openfga: initialize: %w", err)
	}
	return checker, nil
}

// newAuth wires the OIDC authenticator (Keycloak). In dev (no auth
// config) the service is nil and the security handler is nil; the
// HTTP layer will fall back to AnonSecurity.
func newAuth(ctx context.Context, cfg *config.Config, log *slog.Logger, users *user.Service) (*auth.Service, *auth.SecurityHandler, error) {
	if cfg.Auth.Issuer == "" || cfg.Auth.ClientID == "" {
		return nil, nil, nil
	}
	if cfg.Auth.EncryptionKey == "" {
		return nil, nil, fmt.Errorf("auth: encryption_key required when auth.issuer is set")
	}
	authenticator, err := auth.New(ctx, auth.Config{
		Issuer:         cfg.Auth.Issuer,
		ClientID:       cfg.Auth.ClientID,
		RedirectURI:    cfg.Auth.RedirectURI,
		PostLoginURL:   cfg.Auth.PostLoginURL,
		PostLogoutURL:  cfg.Auth.PostLogoutURL,
		CookieName:     cfg.HTTP.Cookie.Name,
		CookieDomain:   cfg.Auth.CookieDomain,
		CookieSameSite: parseSameSite(cfg.Auth.CookieSameSite),
		EncryptionKey:  cfg.Auth.EncryptionKey,
		SessionTTL:     cfg.Auth.SessionTTL,
		Scopes:         []string{"openid", "profile", "email"},
		Provider:       cfg.Auth.Provider,
	}, users)
	if err != nil {
		return nil, nil, err
	}
	sec := auth.NewSecurityHandler(authenticator)
	return authenticator, sec, nil
}

func parseSameSite(s string) http.SameSite {
	switch s {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}

// newVFS builds the inode-tree manager. vfs has no S3 or HTTP
// dependency: it manages paths, links, and the tree; it
// notifies Garbage when object nodes go away.
func newVFS(repos repositories, garbage *gc.GarbageRecorder, log *slog.Logger) *vfs.Service {
	return vfs.NewService(vfs.ServiceConfig{
		NodeClient:      repos.NodeSvc,
		DriveClient:     repos.DriveSvc,
		GarbageRecorder: garbage,
		Logger:          log,
	})
}

// newUpload builds the S3 lifecycle service. Permission is the
// handler's responsibility; the service is the pure
// presign/complete/delete flow.
func newUpload(repos repositories, store *s3.Client, reg upload.TokenRegistry) *upload.Service {
	return upload.NewService(upload.Config{
		TokenRegistry: reg,
		StorageLookup: repos.DriveSvc,
		NodeLifecycle: repos.NodeSvc,
		ObjectStore:   store,
		Path:          nil, // set below: depends on vfs which depends on Garbage
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

// rootNodeCreator adapts node.Service to drive.RootDirectoryCreator.
type rootNodeCreator struct {
	svc *node.Service
}

func (n *rootNodeCreator) CreateRootDirectory(ctx context.Context) (uuid.UUID, error) {
	root, err := n.svc.CreateDirectory(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return root.ID(), nil
}

// newS3Client builds an upload/s3 client. The same instance is
// shared by upload.Service (presign + delete) and the gc
// tombstone cleaner.
func newS3Client(ctx context.Context, cfg config.StorageConfig, log *slog.Logger) (*s3.Client, error) {
	endpoint := (*string)(nil)
	if cfg.Endpoint != "" {
		endpoint = &cfg.Endpoint
	}
	client, err := s3.NewClient(ctx, s3.Config{
		Region:       cfg.Region,
		Endpoint:     endpoint,
		AccessKey:    cfg.AccessKey,
		SecretKey:    cfg.SecretKey,
		UsePathStyle: cfg.UsePathStyle,
	})
	if err != nil {
		return nil, fmt.Errorf("s3 client: %w", err)
	}
	log.Info("s3 client initialized", "endpoint", endpoint, "region", cfg.Region, "path_style", cfg.UsePathStyle)
	return client, nil
}

func newUploadRegistry(ctx context.Context, cfg config.ValkeyConfig, log *slog.Logger) (upload.TokenRegistry, error) {
	if len(cfg.Addrs) == 0 || cfg.Addrs[0] == "" {
		log.Warn("valkey: no addresses configured; using MemoryRegistry (development only)")
		return upload.NewMemoryRegistry(), nil
	}
	client, err := valkey.NewClient(valkey.ClientOption{
		InitAddress: cfg.Addrs,
		Username:    cfg.User,
		Password:    cfg.Password,
		SelectDB:    cfg.DB,
	})
	if err != nil {
		return nil, fmt.Errorf("valkey client: %w", err)
	}
	log.Info("valkey client initialized", "addrs", cfg.Addrs, "db", cfg.DB)
	return upload.NewValkeyRegistry(client), nil
}
