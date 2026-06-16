package vfs

import (
	"context"
	"fmt"
	"time"

	"github.com/mandacode-labs/mdrive/internal/core/user"
)

// User operations on the vfs Service.

// UpsertUser creates a user from OIDC claims or updates an existing one.
func (s *Service) UpsertUser(ctx context.Context, cmd *user.CreateCommand) (*user.User, error) {
	if cmd.Provider == "" {
		return nil, user.ErrProviderRequired
	}
	if cmd.ProviderID == "" {
		return nil, user.ErrProviderIDRequired
	}
	if cmd.Name == "" {
		return nil, user.ErrNameRequired
	}

	existing, err := s.user.GetByProviderID(ctx, cmd.Provider, cmd.ProviderID)
	if err != nil {
		return nil, fmt.Errorf("vfs: lookup user: %w", err)
	}
	if existing != nil {
		if existing.Name() != cmd.Name || emailDiffers(existing, cmd) {
			updated := user.NewUser(
				existing.ID(), existing.PublicID(), cmd.Name, cmd.Email,
				existing.Provider(), existing.ProviderID(),
				existing.CreatedAt(), time.Now(),
			)
			saved, err := s.user.Update(ctx, updated)
			if err != nil {
				return nil, fmt.Errorf("vfs: update user: %w", err)
			}
			return saved, nil
		}
		return existing, nil
	}

	created, err := s.user.Create(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("vfs: create user: %w", err)
	}
	return created, nil
}

// GetUser returns a user by private ID.
func (s *Service) GetUser(ctx context.Context, id string) (*user.User, error) {
	u, err := s.user.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, user.ErrNotFound
	}
	return u, nil
}

// GetUserByPublicID returns a user by public ID.
func (s *Service) GetUserByPublicID(ctx context.Context, publicID string) (*user.User, error) {
	u, err := s.user.GetByPublicID(ctx, publicID)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, user.ErrNotFound
	}
	return u, nil
}

// UserExists checks if a user exists.
func (s *Service) UserExists(ctx context.Context, id string) (bool, error) {
	u, err := s.user.GetByID(ctx, id)
	if err != nil {
		return false, err
	}
	return u != nil, nil
}

func emailDiffers(u *user.User, cmd *user.CreateCommand) bool {
	if cmd.Email == nil && u.Email() == nil {
		return false
	}
	if cmd.Email == nil || u.Email() == nil {
		return true
	}
	return *u.Email() != *cmd.Email
}
