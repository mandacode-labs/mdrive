package handler

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/core/user"
	api "github.com/mandacode-labs/mdrive/pkg/api"
)

// --- User handlers ---

func (h *Handler) UpsertUser(ctx context.Context, req api.OptUpsertUserReq) (*api.User, error) {
	r := req.Value
	var email *string
	if r.Email.Set {
		e := r.Email.Value
		email = &e
	}
	u, err := h.vfs.UpsertUser(ctx, &user.CreateCommand{
		Name:       r.Name,
		Email:      email,
		Provider:   r.Provider,
		ProviderID: r.ProviderID,
	})
	if err != nil {
		return nil, err
	}
	return userToAPI(u), nil
}

func (h *Handler) GetUser(ctx context.Context) (*api.User, error) {
	u, err := h.vfs.GetUser(ctx, h.userID(ctx))
	if err != nil {
		return nil, err
	}
	return userToAPI(u), nil
}

func userToAPI(u *user.User) *api.User {
	if u == nil {
		return nil
	}
	return &api.User{
		ID:        apistr(u.ID()),
		PublicID:  apistr(u.PublicID()),
		Name:      apistr(u.Name()),
		Email:     apistrPtr(u.Email()),
		CreatedAt: api.OptDateTime{Value: u.CreatedAt(), Set: true},
		UpdatedAt: api.OptDateTime{Value: u.UpdatedAt(), Set: true},
	}
}
