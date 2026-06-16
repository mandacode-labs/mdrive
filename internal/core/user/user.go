// Package user provides the user domain: identity from OIDC providers.
package user

import (
	"context"
	"time"

	"github.com/mandacode-labs/mdrive/internal/errors"
	"github.com/mandacode-labs/mdrive/internal/idgen"
)

// User is an externally-authenticated user.
// Identity is established by (provider, provider_id) which maps to the OIDC `sub` claim.
type User struct {
	id         string
	publicID   string
	name       string
	email      *string
	provider   string
	providerID string
	createdAt  time.Time
	updatedAt  time.Time
}

// NewUser creates a new User.
func NewUser(
	id string,
	publicID string,
	name string,
	email *string,
	provider string,
	providerID string,
	createdAt time.Time,
	updatedAt time.Time,
) *User {
	return &User{
		id:         id,
		publicID:   publicID,
		name:       name,
		email:      email,
		provider:   provider,
		providerID: providerID,
		createdAt:  createdAt,
		updatedAt:  updatedAt,
	}
}

// Getters.
func (u *User) ID() string         { return u.id }
func (u *User) PublicID() string   { return u.publicID }
func (u *User) Name() string       { return u.name }
func (u *User) Email() *string     { return u.email }
func (u *User) Provider() string   { return u.provider }
func (u *User) ProviderID() string { return u.providerID }
func (u *User) CreatedAt() time.Time { return u.createdAt }
func (u *User) UpdatedAt() time.Time { return u.updatedAt }

// CreateCommand for upserting a user from OIDC claims.
type CreateCommand struct {
	Name       string
	Email      *string
	Provider   string
	ProviderID string
}

// Repository is the data-access contract for users.
type Repository interface {
	Create(ctx context.Context, cmd *CreateCommand) (*User, error)
	GetByID(ctx context.Context, id string) (*User, error)
	GetByPublicID(ctx context.Context, publicID string) (*User, error)
	GetByProviderID(ctx context.Context, provider, providerID string) (*User, error)
	Update(ctx context.Context, u *User) (*User, error)
	Delete(ctx context.Context, id string) error
}

// Exister checks if a user exists. Used by other packages (e.g., drive) to verify
// ownership without depending on the full user.Service.
type Exister interface {
	Exists(ctx context.Context, id string) (bool, error)
}

// Service implements user identity operations.
type Service struct {
	repo Repository
}

// NewService creates a new Service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// UpsertFromOIDC creates a user from OIDC claims, or updates the existing one.
// Identity is (provider, provider_id).
func (s *Service) UpsertFromOIDC(ctx context.Context, cmd *CreateCommand) (*User, error) {
	if cmd.Provider == "" {
		return nil, errors.BadRequest("provider is required")
	}
	if cmd.ProviderID == "" {
		return nil, errors.BadRequest("provider_id is required")
	}
	if cmd.Name == "" {
		return nil, errors.BadRequest("name is required")
	}

	existing, err := s.repo.GetByProviderID(ctx, cmd.Provider, cmd.ProviderID)
	if err != nil {
		return nil, errors.WrapInternal(err, "lookup user")
	}
	if existing != nil {
		// Update name/email if changed.
		if nameChanged(existing, cmd) || emailChanged(existing, cmd) {
			updated := NewUser(
				existing.ID(),
				existing.PublicID(),
				cmd.Name,
				cmd.Email,
				existing.Provider(),
				existing.ProviderID(),
				existing.CreatedAt(),
				time.Now(),
			)
			saved, err := s.repo.Update(ctx, updated)
			if err != nil {
				return nil, errors.WrapInternal(err, "update user")
			}
			return saved, nil
		}
		return existing, nil
	}

	// Create new user with generated IDs.
	created, err := s.repo.Create(ctx, cmd)
	if err != nil {
		return nil, errors.WrapInternal(err, "create user")
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
		return nil, errors.NotFound("user not found")
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
		return nil, errors.NotFound("user not found")
	}
	return u, nil
}

// Exists checks if a user with the given ID exists.
func (s *Service) Exists(ctx context.Context, id string) (bool, error) {
	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return false, err
	}
	return u != nil, nil
}

func nameChanged(u *User, cmd *CreateCommand) bool {
	return u.Name() != cmd.Name
}

func emailChanged(u *User, cmd *CreateCommand) bool {
	if cmd.Email == nil && u.Email() == nil {
		return false
	}
	if cmd.Email == nil || u.Email() == nil {
		return true
	}
	return *u.Email() != *cmd.Email
}

// GenerateID returns a new ULID for use as user ID.
func GenerateID() string {
	return idgen.Generate()
}
