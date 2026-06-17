package handler

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/core/user"
	apiv1 "github.com/mandacode-labs/mdrive/pkg/apiv1"
)

// --- User handlers ---

func (h *Handler) UpsertUser(ctx context.Context, req apiv1.OptUpsertUserReq) error {
	r := req.Value
	var email *string
	if r.Email.Set {
		e := r.Email.Value
		email = &e
	}
	_, err := h.vfs.UpsertUser(ctx, &user.CreateCommand{
		Name:       r.Name,
		Email:      email,
		Provider:   r.Provider,
		ProviderID: r.ProviderID,
	})
	return err
}

func (h *Handler) GetUser(ctx context.Context) (*apiv1.User, error) {
	u, err := h.vfs.GetUser(ctx, h.userID(ctx))
	if err != nil {
		return nil, err
	}
	return userToAPI(u), nil
}

func userToAPI(u *user.User) *apiv1.User {
	if u == nil {
		return nil
	}
	return &apiv1.User{
		ID:        apistr(u.ID()),
		PublicID:  apistr(u.PublicID()),
		Name:      apistr(u.Name()),
		Email:     apistrPtr(u.Email()),
		CreatedAt: apiv1.OptDateTime{Value: u.CreatedAt(), Set: true},
		UpdatedAt: apiv1.OptDateTime{Value: u.UpdatedAt(), Set: true},
	}
}
