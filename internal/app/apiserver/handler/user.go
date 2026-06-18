package handler

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/app/apputils"
	"github.com/mandacode-labs/mdrive/internal/core/user"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

// userToAPI converts a domain user to an API user.
func userToAPI(u *user.User) *api.User {
	if u == nil {
		return nil
	}
	return &api.User{
		ID:        apputils.OptString(u.ID()),
		PublicID:  apputils.OptString(u.PublicID()),
		Name:      apputils.OptString(u.Name()),
		Email:     apputils.OptStringPtr(u.Email()),
		CreatedAt: api.OptDateTime{Value: u.CreatedAt(), Set: true},
		UpdatedAt: api.OptDateTime{Value: u.UpdatedAt(), Set: true},
	}
}

func (h *Handler) UpsertUser(ctx context.Context, req api.OptUpsertUserReq) error {
	r := req.Value
	var email *string
	if r.Email.Set {
		e := r.Email.Value
		email = &e
	}
	uid := h.userID(ctx)
	_, err := h.vfs.UpsertUser(ctx, uid, &user.CreateCommand{
		Name:       r.Name,
		Email:      email,
		Provider:   r.Provider,
		ProviderID: r.ProviderID,
	})
	return err
}

func (h *Handler) GetUser(ctx context.Context) (*api.User, error) {
	u, err := h.vfs.GetUser(ctx, h.userID(ctx), h.userID(ctx))
	if err != nil {
		return nil, err
	}
	return userToAPI(u), nil
}

