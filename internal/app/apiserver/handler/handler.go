package handler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ogen-go/ogen/ogenerrors"

	"github.com/mandacode-labs/mdrive/internal/app/apiserver/middleware"
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
	cookieConfig   CookieConfig
	defaultStorage drive.StorageConfig
	presignTTL     time.Duration
	healthDeps     HealthDeps
}

type CookieConfig = config.CookieConfig

func New(fs FSClient, drive DriveClient, users UserClient, upload UploadClient, authorizer permission.Authorizer, redirectURI string, opts ...Option) *Handler {
	h := &Handler{
		fs:          fs,
		drive:       drive,
		users:       users,
		upload:      upload,
		authorizer:  authorizer,
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

func (h *Handler) requirePerm(ctx context.Context, perm permission.Action, driveID string) error {
	allowed, err := h.authorizer.Check(ctx, h.userID(ctx), perm, permission.ObjectTypeDrive, driveID)
	if err != nil {
		return errorx.Wrap(err, fmt.Sprintf("permission: check (perm=%s, type=%s, id=%s)", perm, permission.ObjectTypeDrive, driveID), errorx.KindUnavailable)
	}
	if !allowed {
		return errorx.New(errorx.KindPermissionDenied, fmt.Sprintf("permission: denied (perm=%s, type=%s, id=%s)", perm, permission.ObjectTypeDrive, driveID))
	}
	return nil
}

// NewError is the ogen WithErrorHandler fallback. Reached for
// errors that bypass the middleware chain — currently the
// SecurityError path. Unwraps to the inner errorx when present
// so the status reflects the kind; otherwise falls back to
// SecurityError.Code() (401) and an unauthenticated code.
func (h *Handler) NewError(_ context.Context, err error) *api.ErrorStatusCode {
	var sec *ogenerrors.SecurityError
	if errors.As(err, &sec) {
		if sec.Err != nil {
			var de errorx.Error
			if errors.As(sec.Err, &de) {
				return middleware.KindToCode(sec.Err)
			}
		}
		return &api.ErrorStatusCode{
			StatusCode: sec.Code(),
			Response: api.Error{
				Code:    api.ErrorCodeUnauthenticated,
				Message: err.Error(),
			},
		}
	}
	return &api.ErrorStatusCode{
		StatusCode: 500,
		Response: api.Error{
			Code:    api.ErrorCodeInternal,
			Message: err.Error(),
		},
	}
}

// AuthLogin is a stub. The AuthPassthrough middleware routes
// /auth/login to the auth Service before ogen sees it; this
// method only runs if the middleware is misconfigured. Returning a
// redirect keeps clients that follow the spec sane in that case.
func (h *Handler) AuthLogin(ctx context.Context) (*api.AuthLoginFound, error) {
	return &api.AuthLoginFound{Location: h.redirectURI}, nil
}

// AuthCallback is a stub. Handled by the auth Service in the
// AuthPassthrough middleware. Kept for spec completeness.
func (h *Handler) AuthCallback(ctx context.Context, params api.AuthCallbackParams) (*api.AuthCallbackFound, error) {
	return &api.AuthCallbackFound{Location: h.redirectURI}, nil
}

// AuthLogout is a stub. Handled by the auth Service in the
// AuthPassthrough middleware. Kept for spec completeness.
func (h *Handler) AuthLogout(ctx context.Context) (*api.AuthLogoutFound, error) {
	return &api.AuthLogoutFound{Location: h.redirectURI}, nil
}

var _ api.Handler = (*Handler)(nil)
