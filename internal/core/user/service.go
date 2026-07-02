package user

import (
	"context"
	"time"

	"github.com/mandacode-labs/mdrive/internal/errorx"
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
		return nil, errorx.New(errorx.KindBadRequest, "user: provider is required")
	}
	if cmd.ProviderID == "" {
		return nil, errorx.New(errorx.KindBadRequest, "user: provider_id is required")
	}
	if cmd.Name == "" {
		return nil, errorx.New(errorx.KindBadRequest, "user: name is required")
	}

	existing, err := s.repo.GetByProviderID(ctx, cmd.Provider, cmd.ProviderID)
	if err != nil {
		return nil, errorx.Wrap(err, "user: upsert lookup", errorx.KindServiceDegraded)
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
				return nil, errorx.Wrap(err, "user: upsert update", errorx.KindServiceDegraded)
			}
			return saved, nil
		}
		return existing, nil
	}

	created, err := s.repo.Create(ctx, cmd)
	if err != nil {
		return nil, errorx.Wrap(err, "user: upsert create", errorx.KindServiceDegraded)
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
		return nil, errorx.New(errorx.KindNotFound, "user: not found (id="+id+")")
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
		return nil, errorx.New(errorx.KindNotFound, "user: not found (public_id="+publicID+")")
	}
	return u, nil
}

// GetByProviderID returns a user by (provider, providerID).
func (s *Service) GetByProviderID(ctx context.Context, provider, providerID string) (*User, error) {
	return s.repo.GetByProviderID(ctx, provider, providerID)
}

// Update updates an existing user.
func (s *Service) Update(ctx context.Context, u *User) (*User, error) {
	return s.repo.Update(ctx, u)
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
