package handler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ogen-go/ogen/ogenerrors"

	"github.com/mandacode-labs/mdrive/internal/app/apiserver/middleware"
	"github.com/mandacode-labs/mdrive/internal/auth"
	"github.com/mandacode-labs/mdrive/internal/config"
	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/user"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/permission"
	"github.com/mandacode-labs/mdrive/internal/upload"
	"github.com/mandacode-labs/mdrive/internal/vfs"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

type Handler struct {
	fs             vfs.Service
	drive          drive.Service
	users          user.Service
	upload         upload.Service
	authorizer     permission.Authorizer
	redirectURI    string
	cookieConfig   CookieConfig
	defaultStorage drive.StorageConfig
	presignTTL     time.Duration
	healthDeps     HealthDeps
}

type CookieConfig = config.CookieConfig

func New(fs vfs.Service, drive drive.Service, users user.Service, upload upload.Service, authorizer permission.Authorizer, redirectURI string, opts ...Option) *Handler {
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
