package user

import (
	"context"
	"fmt"
	"time"
)

// Service provides domain-level user operations.
type Service struct {
	repo Repository
}

// NewService creates a new Service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// UpsertFromOIDC creates or updates a user from OIDC claims.
func (s *Service) UpsertFromOIDC(ctx context.Context, cmd *CreateCommand) (*User, error) {
	if cmd.Provider == "" {
		return nil, ErrProviderRequired
	}
	if cmd.ProviderID == "" {
		return nil, ErrProviderIDRequired
	}
	if cmd.Name == "" {
		return nil, ErrNameRequired
	}

	existing, err := s.repo.GetByProviderID(ctx, cmd.Provider, cmd.ProviderID)
	if err != nil {
		return nil, fmt.Errorf("user: lookup: %w", err)
	}
	if existing != nil {
		if existing.Name() != cmd.Name || emailDiffers(existing, cmd) {
			updated := NewUser(
				existing.ID(), existing.PublicID(), cmd.Name, cmd.Email,
				existing.Provider(), existing.ProviderID(),
				existing.CreatedAt(), time.Now(),
			)
			saved, err := s.repo.Update(ctx, updated)
			if err != nil {
				return nil, fmt.Errorf("user: update: %w", err)
			}
			return saved, nil
		}
		return existing, nil
	}

	created, err := s.repo.Create(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("user: create: %w", err)
	}
	return created, nil
}

// GetByID returns a user by private ID.
func (s *Service) GetByID(ctx context.Context, id string) (*User, error) {
	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, ErrNotFound
	}
	return u, nil
}

// GetByPublicID returns a user by public ID.
func (s *Service) GetByPublicID(ctx context.Context, publicID string) (*User, error) {
	u, err := s.repo.GetByPublicID(ctx, publicID)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, ErrNotFound
	}
	return u, nil
}

// Exists checks if a user exists.
func (s *Service) Exists(ctx context.Context, id string) (bool, error) {
	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return false, err
	}
	return u != nil, nil
}

func emailDiffers(u *User, cmd *CreateCommand) bool {
	if cmd.Email == nil && u.Email() == nil {
		return false
	}
	if cmd.Email == nil || u.Email() == nil {
		return true
	}
	return *u.Email() != *cmd.Email
}
