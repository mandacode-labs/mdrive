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
	ResolveForPermission(ctx context.Context, driveID, path string) (vfs.ResolvedRef, error)
	Mkdir(ctx context.Context, driveID, path string) (*node.Node, error)
	Touch(ctx context.Context, driveID, path string) (*node.Node, error)
	Rm(ctx context.Context, driveID string, paths []string, recursive bool) error
	Mv(ctx context.Context, srcDriveID string, srcPaths []string, dstDriveID, dstPath string) error
	Ls(ctx context.Context, driveID, path string) (node.DirContent, error)
	Cat(ctx context.Context, driveID, path string) ([]byte, error)
	Write(ctx context.Context, driveID, path, content string) error
	WriteLarge(ctx context.Context, driveID, path string, obj node.ObjectContent, size int64) error
	Stat(ctx context.Context, driveID, path string) (*node.Node, error)
	Symlink(ctx context.Context, driveID, target, linkPath string) (*node.Node, error)
}

// DriveClient is the consumer-declared interface for drive CRUD.
type DriveClient interface {
	Create(ctx context.Context, actorID string, name, description string, cfg drive.StorageConfig) (*drive.Drive, uuid.UUID, error)
	Get(ctx context.Context, actorID, id string) (*drive.Drive, error)
	GetStorage(ctx context.Context, actorID, driveID string) (*drive.Storage, error)
	Update(ctx context.Context, actorID, id string, name, description *string) (*drive.Drive, error)
	Delete(ctx context.Context, actorID, id string) error
	Restore(ctx context.Context, actorID, id string) (*drive.Drive, error)
	ListByOwner(ctx context.Context, actorID string) ([]*drive.Drive, error)
	ListDeleted(ctx context.Context, isAdmin bool) ([]*drive.Drive, error)
}

// UserClient is the consumer-declared interface for user CRUD.
// vfs no longer manages users: user operations live in core/user
// and are exposed directly to the handler.
type UserClient = *user.Service

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

// ErrPermission is the handler-level permission-denied error.
// vfs and drive both return their own ErrPermission; the handler
// re-uses them when propagating.
var ErrPermission = permission.ErrPermission

type Handler struct {
	vfs            FSClient
	drive          DriveClient
	users          UserClient
	upload         UploadClient
	perm           permission.Checker
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

func New(fs FSClient, drive DriveClient, users UserClient, upload UploadClient, perm permission.Checker, getUser func(context.Context) (string, bool), opts ...Option) *Handler {
	h := &Handler{
		vfs:     fs,
		drive:   drive,
		users:   users,
		upload:  upload,
		perm:    perm,
		getUser: getUser,
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

// checkEdit returns nil if the user has edit permission on driveID,
// ErrPermission otherwise. Used before any write-side FS op.
func (h *Handler) checkEdit(ctx context.Context, userID, driveID string) error {
	if h.perm == nil {
		return nil
	}
	allowed, err := h.perm.Check(ctx, userID, permission.PermissionEdit, permission.ObjectTypeDrive, driveID)
	if err != nil {
		return err
	}
	if !allowed {
		return permission.ErrPermission
	}
	return nil
}

// checkViewAfterResolve extends a permission check to the drive the
// path ultimately resolves to (which may differ from driveID if a
// mount was crossed). The caller invokes vfs.Resolve first, then
// passes the resulting drive to this check.
func (h *Handler) checkViewAfterResolve(ctx context.Context, userID, resolvedDriveID string) error {
	if h.perm == nil {
		return nil
	}
	allowed, err := h.perm.Check(ctx, userID, permission.PermissionView, permission.ObjectTypeDrive, resolvedDriveID)
	if err != nil {
		return err
	}
	if !allowed {
		return permission.ErrPermission
	}
	return nil
}

// Compile-time checks.
var _ api.Handler = (*Handler)(nil)
var _ AuthClient = (*auth.Service)(nil)
