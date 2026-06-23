package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/zitadel/oidc/v3/pkg/oidc"

	"github.com/mandacode-labs/mdrive/internal/auth"
	"github.com/mandacode-labs/mdrive/internal/auth/session"
	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/core/user"
	"github.com/mandacode-labs/mdrive/internal/vfs"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

// FSClient is the consumer-declared interface for filesystem
// operations on a vfs service. The handler depends on this
// interface (rather than vfs.Service directly) so the wiring
// stays decoupled from the vfs implementation and tests can
// supply fakes per-method.
type FSClient interface {
	Mkdir(ctx context.Context, userID, driveID, path string) (*node.Node, error)
	Touch(ctx context.Context, userID, driveID, path string) (*node.Node, error)
	Rm(ctx context.Context, userID, driveID string, paths []string, recursive bool) error
	Mv(ctx context.Context, userID, srcDriveID string, srcPaths []string, dstDriveID, dstPath string) error
	Ls(ctx context.Context, userID, driveID, path string) (node.DirContent, error)
	Cat(ctx context.Context, userID, driveID, path string) ([]byte, error)
	Write(ctx context.Context, userID, driveID, path, content string) error
	WriteLarge(ctx context.Context, userID, driveID, path string, obj node.ObjectContent, size int64) error
	Stat(ctx context.Context, userID, driveID, path string) (*node.Node, error)
	Symlink(ctx context.Context, userID, driveID, target, linkPath string) (*node.Node, error)
	InitiateUpload(ctx context.Context, userID, driveID, destPath string, contentType *string, contentLength *int64, expiry time.Duration) (vfs.PresignInfo, error)
	CompleteUpload(ctx context.Context, userID, driveID, uploadID string, contentLength int64, checksum *string) (*node.Node, error)
	PresignDownload(ctx context.Context, userID, driveID, path string, expiry time.Duration) (vfs.PresignInfo, error)
	UpsertUser(ctx context.Context, actorID string, cmd *user.CreateCommand) (*user.User, error)
	GetUser(ctx context.Context, actorID, id string) (*user.User, error)
}

// DriveClient is the consumer-declared interface for drive CRUD.
// It is implemented by vfs/drive.Service (a subpackage of vfs
// that composes the core drive.Service with permission checks).
// Splitting this out of FSClient keeps the handler's dependency
// on filesystem code minimal: drive CRUD has nothing to do with
// node/permission/S3 I/O.
type DriveClient interface {
	Create(ctx context.Context, actorID string, name, description string, cfg drive.StorageConfig) (*drive.Drive, uuid.UUID, error)
	Get(ctx context.Context, actorID, id string) (*drive.Drive, error)
	GetStorage(ctx context.Context, actorID, driveID string) (*drive.Storage, error)
	Update(ctx context.Context, actorID, id string, name, description *string) (*drive.Drive, error)
	Delete(ctx context.Context, actorID, id string) error
	Restore(ctx context.Context, actorID, id string) (*drive.Drive, error)
	ListByOwner(ctx context.Context, actorID string) ([]*drive.Drive, error)
	ListDeleted(ctx context.Context) ([]*drive.Drive, error)
}

// AuthClient is the consumer-declared interface for authentication operations.
type AuthClient interface {
	ExchangeJWT(ctx context.Context, assertion string) (*oidc.Tokens[*oidc.IDTokenClaims], error)
	ExchangeCode(ctx context.Context, code, redirectURI, codeVerifier string) (*oidc.Tokens[*oidc.IDTokenClaims], error)
	AuthorizeURL(ctx context.Context, provider, redirectURI, state, codeChallenge string) (string, error)
	VerifyIDToken(ctx context.Context, raw string) (*oidc.IDTokenClaims, error)
	CreateSession(ctx context.Context, userID, provider string, isAdmin bool) (*session.Session, error)
	DeleteSession(ctx context.Context, id string) error
	StorePKCE(ctx context.Context, state, verifier string) error
	GetPKCE(ctx context.Context, state string) (string, error)
}

type Handler struct {
	vfs            FSClient
	drive          DriveClient
	getUser        func(context.Context) (string, bool)
	auth           AuthClient
	frontendURL    string
	cookieConfig   CookieConfig
	defaultStorage drive.StorageConfig
	healthDeps     HealthDeps
}

type CookieConfig struct {
	Name     string
	Path     string
	Secure   bool
	HttpOnly bool
	SameSite http.SameSite
}

func New(fs FSClient, drive DriveClient, getUser func(context.Context) (string, bool), opts ...Option) *Handler {
	h := &Handler{vfs: fs, drive: drive, getUser: getUser}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

type Option func(*Handler)

func WithDefaultStorage(cfg drive.StorageConfig) Option {
	return func(h *Handler) {
		h.defaultStorage = cfg
	}
}

func WithHealthDeps(deps HealthDeps) Option {
	return func(h *Handler) {
		h.healthDeps = deps
	}
}

func WithCookie(cfg CookieConfig) Option {
	return func(h *Handler) {
		h.cookieConfig = cfg
	}
}

func (h *Handler) WithAuth(a AuthClient, frontendURL string) {
	h.auth = a
	h.frontendURL = frontendURL
}

func (h *Handler) userID(ctx context.Context) string {
	if id, ok := h.getUser(ctx); ok {
		return id
	}
	if h.auth != nil {
		sess := auth.SessionFromContext(ctx)
		if sess != nil {
			return sess.UserID
		}
	}
	return ""
}

// Compile-time checks.
var _ api.Handler = (*Handler)(nil)
var _ AuthClient = (*auth.Service)(nil)
