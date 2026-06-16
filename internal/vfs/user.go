package vfs

import (
	"context"
	"fmt"

	"github.com/mandacode-labs/mdrive/internal/core/user"
)

// User operations — delegates to the user domain service.

// UpsertUser creates or updates a user from OIDC claims.
func (s *Service) UpsertUser(ctx context.Context, cmd *user.CreateCommand) (*user.User, error) {
	u, err := s.userSvc.UpsertFromOIDC(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("vfs: %w", err)
	}
	return u, nil
}

// GetUser returns a user by private ID.
func (s *Service) GetUser(ctx context.Context, id string) (*user.User, error) {
	return s.userSvc.GetByID(ctx, id)
}

// GetUserByPublicID returns a user by public ID.
func (s *Service) GetUserByPublicID(ctx context.Context, publicID string) (*user.User, error) {
	return s.userSvc.GetByPublicID(ctx, publicID)
}

// UserExists checks if a user exists.
func (s *Service) UserExists(ctx context.Context, id string) (bool, error) {
	return s.userSvc.Exists(ctx, id)
}
