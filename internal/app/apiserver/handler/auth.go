package handler

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/logx"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

var (
	ErrUnauthenticated   = errorx.New(errorx.KindUnauthenticated, "auth: not authenticated")
	ErrServiceDegraded   = errorx.New(errorx.KindServiceDegraded, "auth: upstream failure")
	ErrUserNotFoundLocal = errorx.New(errorx.KindNotFound, "auth: user not found")
)

// AuthMe returns the authenticated user. Errors are returned as
// *api.Error so ogen picks them up via the AuthMeRes interface;
// the status code and wire body come from apierr.FromError so the
// same errorx.Kind produces the same response as the WithErrorHandler.
func (h *Handler) AuthMe(ctx context.Context) (api.AuthMeRes, error) {
	uid := h.userID(ctx)
	logx.Debug(ctx, "handler.auth.me.enter", slog.String("user_id", uid))
	if uid == "" {
		logx.Debug(ctx, "handler.auth.me.no_session")
		return apiErr(errorx.New(errorx.KindUnauthenticated, "auth: not authenticated")), nil
	}
	u, err := h.users.GetByID(ctx, uid)
	if err != nil {
		logx.Debug(ctx, "handler.auth.me.lookup_err",
			slog.String("user_id", uid),
			slog.String("err", err.Error()),
		)
		return apiErr(errorx.New(errorx.KindServiceDegraded, fmt.Sprintf("auth: user lookup failed (uid=%s, err=%v)", uid, err))), nil
	}
	if u == nil {
		logx.Debug(ctx, "handler.auth.me.not_found", slog.String("user_id", uid))
		return apiErr(errorx.New(errorx.KindNotFound, "auth: user not found")), nil
	}
	logx.Debug(ctx, "handler.auth.me.ok", slog.String("user_id", u.ID()))
	return userToAPI(u), nil
}

// apiErr converts an errorx error to the api.Error wire format.
// Returned as a pointer so the AuthMeRes interface dispatches to
// the error branch (not the *User branch) and ogen emits the
// correct status code.
func apiErr(err error) *api.Error {
	_, e := FromError(err)
	return &api.Error{
		Code:    api.ErrorCode(e.Code),
		Message: e.Message,
	}
}
