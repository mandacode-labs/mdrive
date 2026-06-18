package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/core/user"
)

// UpsertUser creates or updates a user from OIDC claims.
// Users may only mutate their own profile.
func (s *Service) UpsertUser(ctx context.Context, actorID string, cmd *user.CreateCommand) (*user.User, error) {
	if actorID != "" && actorID != cmd.ProviderID {
		return nil, ErrPermission
	}
	return s.User.UpsertFromOIDC(ctx, cmd)
}

// GetUser returns a user by private ID.
// Users may only read their own profile.
func (s *Service) GetUser(ctx context.Context, actorID, id string) (*user.User, error) {
	if actorID != "" && actorID != id {
		return nil, ErrPermission
	}
	return s.User.GetByID(ctx, id)
}
