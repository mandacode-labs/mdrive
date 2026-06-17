package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/core/user"
)

// UpsertUser creates or updates a user from OIDC claims.
func (s *Service) UpsertUser(ctx context.Context, cmd *user.CreateCommand) (*user.User, error) {
	return s.User.UpsertFromOIDC(ctx, cmd)
}

// GetUser returns a user by private ID.
func (s *Service) GetUser(ctx context.Context, id string) (*user.User, error) {
	return s.User.GetByID(ctx, id)
}
