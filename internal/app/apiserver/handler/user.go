package handler

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/core/user"
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
	if _, err := h.users.UpsertFromOIDC(ctx, &user.CreateCommand{
		Name:       r.Name,
		Email:      email,
		Provider:   r.Provider,
		ProviderID: r.ProviderID,
	}); err != nil {
		return nil, err
	}
	return &api.UpsertUserOK{}, nil
}

func (h *Handler) GetUser(ctx context.Context) (api.GetUserRes, error) {
	uid := h.userID(ctx)
	u, err := h.users.GetByID(ctx, uid)
	if err != nil {
		return nil, err
	}
	return userToAPI(u), nil
}
