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
	ErrServiceDegraded   = errorx.New(errorx.KindUnavailable, "auth: upstream failure")
	ErrUserNotFoundLocal = errorx.New(errorx.KindNotFound, "auth: user not found")
)

// AuthMe returns the authenticated user. On failure returns
// (nil, err) so the error middleware converts the errorx to the
// API response. The 401/404/503 status is set by Kind, not by
// the handler.
func (h *Handler) AuthMe(ctx context.Context) (api.AuthMeRes, error) {
	uid := h.userID(ctx)
	logx.Debug(ctx, "handler.auth.me.enter", slog.String("user_id", uid))
	if uid == "" {
		logx.Debug(ctx, "handler.auth.me.no_session")
		return nil, errorx.New(errorx.KindUnauthenticated, "auth: not authenticated")
	}
	u, err := h.users.GetByID(ctx, uid)
	if err != nil {
		logx.Debug(ctx, "handler.auth.me.lookup_err",
			slog.String("user_id", uid),
			slog.String("err", err.Error()),
		)
		return nil, errorx.Wrap(err, fmt.Sprintf("auth: user lookup failed (uid=%s)", uid), errorx.KindUnavailable)
	}
	if u == nil {
		logx.Debug(ctx, "handler.auth.me.not_found", slog.String("user_id", uid))
		return nil, errorx.New(errorx.KindNotFound, "auth: user not found")
	}
	logx.Debug(ctx, "handler.auth.me.ok", slog.String("user_id", u.ID()))
	return userToAPI(u), nil
}
