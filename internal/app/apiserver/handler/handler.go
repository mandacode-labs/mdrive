package handler

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/zitadel/oidc/v3/pkg/oidc"

	"github.com/mandacode-labs/mdrive/internal/auth"
	"github.com/mandacode-labs/mdrive/internal/auth/session"
	"github.com/mandacode-labs/mdrive/internal/config"
	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/core/user"
	"github.com/mandacode-labs/mdrive/internal/permission"
	"github.com/mandacode-labs/mdrive/internal/upload"
	"github.com/mandacode-labs/mdrive/internal/vfs"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

// FSClient is the consumer-declared interface for filesystem
// operations on a vfs service. vfs is filesystem-only: it does
// not know about users or permissions. The handler is
// responsible for permission checks before calling these
// methods.
type FSClient interface {
	ResolveForPermission(ctx context.Context, driveID, path string) (vfs.PartialResolution, error)
	Mkdir(ctx context.Context, driveID, path string) (*node.Node, error)
	Touch(ctx context.Context, driveID, path string) (*node.Node, error)
	Rm(ctx context.Context, driveID string, paths []string, recursive bool) error
	Mv(ctx context.Context, srcDriveID string, srcPaths []string, dstDriveID, dstPath string) error
	Ls(ctx context.Context, driveID, path string) (node.DirContent, error)
	Cat(ctx context.Context, driveID, path string) ([]byte, error)
	Write(ctx context.Context, driveID, path, content string) error
	WriteLarge(ctx context.Context, driveID, path string, obj node.ObjectContent, size int64) error
	Stat(ctx context.Context, driveID, path string) (*node.Node, error)
	Lstat(ctx context.Context, driveID, path string) (vfs.Resolved, error)
	Symlink(ctx context.Context, driveID, target, linkPath string) (*node.Node, error)
	Hardlink(ctx context.Context, driveID, srcPath, linkPath string) (*node.Node, error)
	Mount(ctx context.Context, driveID, mountPath, sourceDriveID string) error
	Unmount(ctx context.Context, driveID, mountPath string) error
}

// DriveClient is the consumer-declared interface for drive CRUD.
type DriveClient interface {
	Create(ctx context.Context, actorID string, name, description string, cfg drive.StorageConfig) (*drive.Drive, uuid.UUID, error)
	GetByID(ctx context.Context, id string) (*drive.Drive, error)
	GetStorage(ctx context.Context, driveID string) (*drive.Storage, error)
	Update(ctx context.Context, id string, name, description string) (*drive.Drive, error)
	Delete(ctx context.Context, id string) error
	Restore(ctx context.Context, id string) (*drive.Drive, error)
	ListByOwner(ctx context.Context, actorID string) ([]*drive.Drive, error)
	ListDeletedForAdmin(ctx context.Context, isAdmin bool, before time.Time, limit int) ([]*drive.Drive, error)
}

// UserClient is the consumer-declared interface for user CRUD.
type UserClient interface {
	UpsertFromOIDC(ctx context.Context, cmd *user.CreateCommand) (*user.User, error)
	GetByID(ctx context.Context, id string) (*user.User, error)
}

// UploadClient is the consumer-declared interface for the
// presigned-upload flow.
type UploadClient interface {
	InitiateUpload(ctx context.Context, userID, driveID, destPath string, contentType *string, contentLength *int64, expiry time.Duration) (upload.PresignInfo, error)
	CompleteUpload(ctx context.Context, userID, driveID, uploadID string, contentLength int64, checksum *string) (*node.Node, error)
	PresignDownload(ctx context.Context, userID, driveID, path string, expiry time.Duration) (upload.PresignInfo, error)
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
	fs             FSClient
	drive          DriveClient
	users          UserClient
	upload         UploadClient
	authorizer     permission.Authorizer
	auth           AuthClient
	redirectURI    string
	cookieConfig   CookieConfig
	defaultStorage drive.StorageConfig
	presignTTL     time.Duration
	healthDeps     HealthDeps
}

// CookieConfig is an alias for config.CookieConfig used in handler
// options. The parsed http.SameSite is materialized via the
// SameSiteMode() method when needed.
type CookieConfig = config.CookieConfig

// New wires the handler. The auth client is optional; when nil,
// requests are expected to arrive without a session (e.g. health
// checks) and any auth-protected endpoint will return an error.
//
// redirectURI is the EXACT callback URL registered in the
// upstream OIDC provider (Zitadel/Keycloak). Pass the value the
// provider holds in its Redirect URIs whitelist — not a base URL.
func New(fs FSClient, drive DriveClient, users UserClient, upload UploadClient, authorizer permission.Authorizer, auth AuthClient, redirectURI string, opts ...Option) *Handler {
	h := &Handler{
		fs:          fs,
		drive:       drive,
		users:       users,
		upload:      upload,
		authorizer:  authorizer,
		auth:        auth,
		redirectURI: redirectURI,
	}
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

func WithPresignTTL(ttl time.Duration) Option {
	return func(h *Handler) {
		h.presignTTL = ttl
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

func (h *Handler) userID(ctx context.Context) string {
	return auth.UserIDFromContext(ctx)
}

// requirePerm centralizes the handler's permission check. All
// writes use ActionEdit; reads use ActionView (the caller
// may need to call ResolveForPermission first if the path may
// cross a mount — see fs.go).
func (h *Handler) requirePerm(ctx context.Context, perm permission.Action, driveID string) error {
	return permission.Require(ctx, h.authorizer, h.userID(ctx), perm, permission.ObjectTypeDrive, driveID)
}

var _ api.Handler = (*Handler)(nil)
var _ AuthClient = (*auth.Service)(nil)
