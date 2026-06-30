package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/auth"
	"github.com/mandacode-labs/mdrive/internal/config"
	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/core/user"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/permission"
	"github.com/mandacode-labs/mdrive/internal/upload"
	"github.com/mandacode-labs/mdrive/internal/vfs"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

type Error = errorx.Error

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

type Handler struct {
	fs             FSClient
	drive          DriveClient
	users          UserClient
	upload         UploadClient
	authorizer     permission.Authorizer
	redirectURI    string
	postLoginURL   string
	cookieConfig   CookieConfig
	defaultStorage drive.StorageConfig
	presignTTL     time.Duration
	healthDeps     HealthDeps
}

type CookieConfig = config.CookieConfig

func New(fs FSClient, drive DriveClient, users UserClient, upload UploadClient, authorizer permission.Authorizer, redirectURI, postLoginURL string, opts ...Option) *Handler {
	h := &Handler{
		fs:           fs,
		drive:        drive,
		users:        users,
		upload:       upload,
		authorizer:   authorizer,
		redirectURI:  redirectURI,
		postLoginURL: postLoginURL,
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

func (h *Handler) requirePerm(ctx context.Context, perm permission.Action, driveID string) error {
	return permission.Require(ctx, h.authorizer, h.userID(ctx), perm, permission.ObjectTypeDrive, driveID)
}

// NewError converts a domain error to an ErrorStatusCode for ogen's
// default error response path. WithErrorHandler is also wired in
// server.go, but ogen's new interface method takes priority for
// default responses — if we returned an empty ErrorStatusCode here,
// every error would default to 200 OK.
func (h *Handler) NewError(_ context.Context, err error) *api.ErrorStatusCode {
	var de errorx.Error
	if errors.As(err, &de) {
		switch de.Kind() {
		case errorx.KindNotFound:
			return &api.ErrorStatusCode{StatusCode: http.StatusNotFound, Response: api.Error{Code: api.ErrorCodeNotFound, Message: "not found"}}
		case errorx.KindConflict:
			return &api.ErrorStatusCode{StatusCode: http.StatusConflict, Response: api.Error{Code: api.ErrorCodeConflict, Message: err.Error()}}
		case errorx.KindBadRequest:
			return &api.ErrorStatusCode{StatusCode: http.StatusBadRequest, Response: api.Error{Code: api.ErrorCodeBadRequest, Message: err.Error()}}
		case errorx.KindForbidden:
			return &api.ErrorStatusCode{StatusCode: http.StatusForbidden, Response: api.Error{Code: api.ErrorCodeForbidden, Message: "permission denied"}}
		case errorx.KindUnauthenticated:
			return &api.ErrorStatusCode{StatusCode: http.StatusUnauthorized, Response: api.Error{Code: api.ErrorCodeUnauthorized, Message: "unauthenticated"}}
		case errorx.KindServiceDegraded:
			return &api.ErrorStatusCode{StatusCode: http.StatusServiceUnavailable, Response: api.Error{Code: api.ErrorCodeInternal, Message: err.Error()}}
		}
	}
	return &api.ErrorStatusCode{StatusCode: http.StatusInternalServerError, Response: api.Error{Code: api.ErrorCodeInternal, Message: "internal error"}}
}

// AuthLogin is a stub. The chart's AuthPassthrough middleware routes
// /auth/login to zitadel-go's authenticator before ogen sees it; this
// method only runs if the middleware is misconfigured. Returning a
// redirect keeps clients that follow the spec sane in that case.
func (h *Handler) AuthLogin(ctx context.Context) (*api.AuthLoginFound, error) {
	return &api.AuthLoginFound{Location: h.redirectURI}, nil
}

// AuthCallback is a stub. Handled by zitadel-go in the chart's
// AuthPassthrough middleware. Kept for spec completeness.
func (h *Handler) AuthCallback(ctx context.Context, params api.AuthCallbackParams) (*api.AuthCallbackFound, error) {
	return &api.AuthCallbackFound{Location: h.redirectURI}, nil
}

// AuthLogout is a stub. Handled by zitadel-go in the chart's
// AuthPassthrough middleware. Kept for spec completeness.
func (h *Handler) AuthLogout(ctx context.Context) (*api.AuthLogoutFound, error) {
	return &api.AuthLogoutFound{Location: h.redirectURI}, nil
}

var _ api.Handler = (*Handler)(nil)
