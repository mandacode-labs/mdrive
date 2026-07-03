package handler

import (
	"context"
	"log/slog"

	"github.com/mandacode-labs/mdrive/internal/core/user"
	"github.com/mandacode-labs/mdrive/internal/logx"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

func userToAPI(u *user.User) *api.User {
	if u == nil {
		return nil
	}
	return &api.User{
		ID:        optString(u.ID()),
		PublicID:  optString(u.PublicID()),
		Name:      optString(u.Name()),
		Email:     optStringPtr(u.Email()),
		CreatedAt: api.OptDateTime{Value: u.CreatedAt(), Set: true},
		UpdatedAt: api.OptDateTime{Value: u.UpdatedAt(), Set: true},
	}
}

func (h *Handler) UpsertUser(ctx context.Context, req api.OptUpsertUserReq) (api.UpsertUserRes, error) {
	r := req.Value
	var email *string
	if r.Email.Set {
		e := r.Email.Value
		email = &e
	}
	logx.Debug(ctx, "handler.user.upsert.enter",
		slog.String("provider", r.Provider),
		slog.String("provider_id", r.ProviderID),
	)
	if _, err := h.users.UpsertFromOIDC(ctx, &user.CreateCommand{
		Name:       r.Name,
		Email:      email,
		Provider:   r.Provider,
		ProviderID: r.ProviderID,
	}); err != nil {
		logx.Debug(ctx, "handler.user.upsert.service_err",
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	logx.Debug(ctx, "handler.user.upsert.ok")
	return &api.UpsertUserOK{}, nil
}

func (h *Handler) GetUser(ctx context.Context) (api.GetUserRes, error) {
	uid := h.userID(ctx)
	logx.Debug(ctx, "handler.user.get.enter", slog.String("user_id", uid))
	u, err := h.users.GetByID(ctx, uid)
	if err != nil {
		logx.Debug(ctx, "handler.user.get.service_err",
			slog.String("user_id", uid),
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	if u == nil {
		logx.Debug(ctx, "handler.user.get.not_found", slog.String("user_id", uid))
	} else {
		logx.Debug(ctx, "handler.user.get.ok", slog.String("user_id", u.ID()))
	}
	return userToAPI(u), nil
}
