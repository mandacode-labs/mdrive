package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/XSAM/otelsql"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/valkey-io/valkey-go"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"github.com/mandacode-labs/mdrive/internal/entx"
	"github.com/mandacode-labs/mdrive/internal/errorx"

	"github.com/mandacode-labs/mdrive/ent"
	"github.com/mandacode-labs/mdrive/internal/app/gc"
	"github.com/mandacode-labs/mdrive/internal/auth"
	"github.com/mandacode-labs/mdrive/internal/config"
	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/core/user"
	cryptopkg "github.com/mandacode-labs/mdrive/internal/crypto"
	"github.com/mandacode-labs/mdrive/internal/logx"
	"github.com/mandacode-labs/mdrive/internal/permission"
	"github.com/mandacode-labs/mdrive/internal/upload"
	"github.com/mandacode-labs/mdrive/internal/upload/s3"
	"github.com/mandacode-labs/mdrive/internal/vfs"
)

// App holds all wired components. Fields are grouped loosely
// into HTTP-serving services (NodeSvc, DriveSvc, ..., Auth),
// background-job services (same, plus Garbage), and lifecycle
// (DB, Ent, Config).
//
// Logging flows through slog.Default — the bootstrap
// (logx.New) installs the configured logger there, and every
// logx.Info/Warn/Error/Request call resolves the default. No
// logger is plumbed through the App because nothing in the
// runtime needs it: handlers, services, and gc runners all use
// the ctx-based logx API.
type App struct {
	Config *config.Config

	NodeSvc      *node.Service
	DriveSvc     *drive.Service
	UserSvc      *user.Service
	OwnerChecker drive.OwnerChecker
	UploadToken  upload.TokenRegistry
	UploadSvc    *upload.Service
	VFS          *vfs.Service
	Garbage      *gc.Recorder
	Auth         *auth.Service
	Security     *auth.Service
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

	// Install the configured logger as slog.Default. Every
	// logx.Info/Warn/Error/Request reads slog.Default, so the
	// rest of the application picks up env/level/format
	// without explicit plumbing.
	logx.New(logx.Config{
		Env:   cfg.App.Env,
		Level: cfg.App.LogLevel,
	})

	db, entClient, err := newInfra(ctx, cfg)
	if err != nil {
		return nil, err
	}

	cipher, err := newCrypto(ctx, cfg)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	repos := newRepositories(entClient, cipher)

	garbage := gc.NewGarbageRecorder(entClient)

	uploadReg, err := newUploadRegistry(ctx, cfg.Valkey)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	s3Client, err := newS3Client(ctx, cfg.Storage)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	permClient, err := newPerm(ctx, cfg)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	authenticator, sec, err := newAuth(ctx, cfg, repos.UserSvc)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	return &App{
		Config:       cfg,
		NodeSvc:      repos.NodeSvc,
		DriveSvc:     repos.DriveSvc,
		UserSvc:      repos.UserSvc,
		OwnerChecker: repos.OwnerChecker,
		UploadToken:  uploadReg,
		UploadSvc:    newUpload(repos, s3Client, uploadReg),
		VFS:          newVFS(repos, garbage),
		Garbage:      garbage,
		Auth:         authenticator,
		Security:     sec,
		Authorizer:   permClient,
		DB:           db,
		Ent:          entClient,
	}, nil
}

// newInfra opens the database and ent client. In development
// the ent schema is auto-created; in production the caller is
// expected to run migrations separately.
func newInfra(ctx context.Context, cfg *config.Config) (*sql.DB, *ent.Client, error) {
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
	logx.Info(ctx, "infra.database.connected",
		slog.String("host", cfg.Database.Host),
		slog.String("database", cfg.Database.Name),
	)

	drv := entsql.OpenDB(dialect.Postgres, db)
	entClient := ent.NewClient(ent.Driver(drv))
	if cfg.App.LogLevel == "debug" {
		entClient = entClient.Debug()
	}

	if cfg.App.Env == "development" {
		if err := entClient.Schema.Create(ctx); err != nil {
			logx.Warn(ctx, "infra.auto_migration.failed",
				slog.String("err", err.Error()),
			)
		}
	}

	return db, entClient, nil
}

// newCrypto returns the at-rest cipher. In production the
// master key is required; in development the NoOp cipher is
// used with a loud startup log so the operator sees the data
// is unprotected.
func newCrypto(ctx context.Context, cfg *config.Config) (cryptopkg.Cipher, error) {
	if cfg.Crypto.MasterKey == "" {
		if cfg.App.Env == "production" {
			return nil, errorx.New(errorx.KindInvalidArgument, "crypto: master key required in production (set CRYPTO_MASTER_KEY or crypto.master_key)")
		}
		logx.Warn(ctx, "crypto.noop.in_use",
			slog.String("note", "drive storage secrets will be stored in plaintext"),
		)
		return cryptopkg.NoOp{}, nil
	}
	cipher, err := cryptopkg.NewAESGCM(cfg.Crypto.MasterKey)
	if err != nil {
		return nil, errorx.Wrap(err, "crypto: initialize aesgcm")
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
	TxMgr        entx.TxManager
}

// newRepositories builds the three core domain services. The
// drive service needs a root creator that wraps node.Service;
// drive verifies owners via the user.Repository directly.
func newRepositories(entClient *ent.Client, cipher cryptopkg.Cipher) repositories {
	txMgr := entx.NewTxManager(entClient)

	nodeRepo := node.NewRepository(entClient)
	nodeSvc := node.NewService(nodeRepo, txMgr)

	userRepo := user.NewRepository(entClient)
	userSvc := user.NewService(userRepo)

	rootCreator := &rootNodeCreator{svc: nodeSvc}
	driveRepo := drive.NewRepository(entClient, cipher)
	driveSvc := drive.NewService(driveRepo, userRepo, rootCreator, txMgr)

	return repositories{
		NodeSvc:      nodeSvc,
		DriveSvc:     driveSvc,
		UserSvc:      userSvc,
		OwnerChecker: userRepo,
		TxMgr:        txMgr,
	}
}

// newPerm returns the OpenFGA Authorizer. The OpenFGA APIURL is
// required: there is no implicit allow-all fallback. Local
// development and tests point the URL at a dev OpenFGA instance
// (docker-compose, integration test, or a mock Authorizer wired
// at the call site).
func newPerm(ctx context.Context, cfg *config.Config) (permission.Authorizer, error) {
	if cfg.OpenFGA.APIURL == "" {
		return nil, errorx.New(errorx.KindInvalidArgument, "openfga: api_url is required (set OPENFGA_APIURL or openfga.api_url)")
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
		return nil, errorx.Wrap(err, fmt.Sprintf("openfga: initialize (api_url=%s)", cfg.OpenFGA.APIURL))
	}
	return checker, nil
}

// newAuth wires the OIDC authenticator (Keycloak). In dev (no auth
// config) the service is nil and the security handler is nil; the
// HTTP layer will fall back to AnonSecurity.
func newAuth(ctx context.Context, cfg *config.Config, users *user.Service) (*auth.Service, *auth.Service, error) {
	if cfg.Auth.Issuer == "" || cfg.Auth.ClientID == "" {
		return nil, nil, nil
	}
	if cfg.Auth.EncryptionKey == "" {
		return nil, nil, errorx.New(errorx.KindInvalidArgument, "auth: encryption_key required when auth.issuer is set")
	}
	authenticator, err := auth.New(ctx, auth.Config{
		Issuer:         cfg.Auth.Issuer,
		ClientID:       cfg.Auth.ClientID,
		ClientSecret:   cfg.Auth.ClientSecret,
		RedirectURI:    cfg.Auth.RedirectURI,
		PostLoginURL:   cfg.Auth.PostLoginURL,
		PostLogoutURL:  cfg.Auth.PostLogoutURL,
		CookieName:     cfg.HTTP.Cookie.Name,
		CookieDomain:   cfg.HTTP.Cookie.Domain,
		CookieSameSite: cfg.HTTP.Cookie.SameSiteMode(),
		CookieSecure:   cfg.HTTP.Cookie.Secure,
		EncryptionKey:  cfg.Auth.EncryptionKey,
		SessionTTL:     cfg.Auth.SessionTTL,
		Scopes:         cfg.Auth.Scopes,
		Provider:       cfg.Auth.Provider,
	}, users)
	if err != nil {
		return nil, nil, err
	}
	sec := authenticator
	return authenticator, sec, nil
}

// newVFS builds the inode-tree manager. vfs has no S3 or HTTP
// dependency: it manages paths, links, and the tree; it
// notifies Garbage when object nodes go away.
func newVFS(repos repositories, garbage *gc.Recorder) *vfs.Service {
	return vfs.NewService(vfs.ServiceConfig{
		NodeClient:      repos.NodeSvc,
		DriveClient:     repos.DriveSvc,
		GarbageRecorder: garbage,
		TxManager:       repos.TxMgr,
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
		TxManager:     repos.TxMgr,
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
//
// Static access_key/secret_key are optional: when both are
// empty we rely on the ambient AWS credential chain (IRSA
// service-account role, EC2/ECS instance profile, env vars).
// Boot only fails when the AWS SDK cannot resolve a region.
func newS3Client(ctx context.Context, cfg config.StorageConfig) (*s3.Client, error) {
	if cfg.Region == "" {
		return nil, errorx.New(errorx.KindInvalidArgument,
			"storage: region is required (set storage.region)")
	}
	if cfg.AccessKey == "" && cfg.SecretKey == "" {
		logx.Info(ctx, "s3.client.using_ambient_credentials",
			slog.String("note", "no static access_key/secret_key; relying on IRSA / instance profile / env"),
		)
	}
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
		return nil, errorx.Wrap(err, fmt.Sprintf("s3: client (region=%s)", cfg.Region))
	}
	logx.Info(ctx, "s3.client.initialized",
		slog.Any("endpoint", endpoint),
		slog.String("region", cfg.Region),
		slog.Bool("path_style", cfg.UsePathStyle),
	)
	return client, nil
}

func newUploadRegistry(ctx context.Context, cfg config.ValkeyConfig) (upload.TokenRegistry, error) {
	if len(cfg.Addrs) == 0 || cfg.Addrs[0] == "" {
		logx.Warn(ctx, "valkey.nop.in_use",
			slog.String("note", "using MemoryRegistry (development only)"),
		)
		return upload.NewMemoryRegistry(), nil
	}
	client, err := valkey.NewClient(valkey.ClientOption{
		InitAddress: cfg.Addrs,
		Username:    cfg.User,
		Password:    cfg.Password,
		SelectDB:    cfg.DB,
	})
	if err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("valkey: client (db=%d)", cfg.DB))
	}
	logx.Info(ctx, "valkey.client.initialized",
		slog.Any("addrs", cfg.Addrs),
		slog.Int("db", cfg.DB),
	)
	return upload.NewValkeyRegistry(client), nil
}
