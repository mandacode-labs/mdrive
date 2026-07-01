package handler

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/auth"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

var ErrUnauthenticated = errorx.New(errorx.KindUnauthenticated, "auth: not authenticated")

func (h *Handler) AuthMe(ctx context.Context) (api.AuthMeRes, error) {
	sess := auth.SessionFromContext(ctx)
	if sess == nil || sess.UserID == "" {
		return nil, ErrUnauthenticated
	}
	u, err := h.users.GetByID(ctx, sess.UserID)
	if err != nil {
		return nil, err
	}
	return userToAPI(u), nil
}