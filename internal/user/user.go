package user

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/errors"
)

// Provider types
const (
	ProviderKeycloak = "keycloak"
	ProviderGoogle   = "google"
)

// IsValidProvider checks if the provider is valid.
func IsValidProvider(provider string) bool {
	switch provider {
	case ProviderKeycloak, ProviderGoogle:
		return true
	default:
		return false
	}
}

// User represents a user in the system.
type User struct {
	id         string
	username   string
	provider   string
	providerID string
	joinDate   time.Time
	createdAt  time.Time
	updatedAt  time.Time
}

// NewUser creates a new User.
func NewUser(
	id string,
	username string,
	provider string,
	providerID string,
	joinDate time.Time,
	createdAt time.Time,
	updatedAt time.Time,
) *User {
	return &User{
		id:         id,
		username:   username,
		provider:   provider,
		providerID: providerID,
		joinDate:   joinDate,
		createdAt:  createdAt,
		updatedAt:  updatedAt,
	}
}

// Getters
func (u *User) ID() string           { return u.id }
func (u *User) Username() string     { return u.username }
func (u *User) Provider() string     { return u.provider }
func (u *User) ProviderID() string   { return u.providerID }
func (u *User) JoinDate() time.Time  { return u.joinDate }
func (u *User) CreatedAt() time.Time { return u.createdAt }
func (u *User) UpdatedAt() time.Time { return u.updatedAt }

// UserService defines the interface for user operations.
type UserService interface {
	Get(ctx context.Context, provider, providerID string) (*User, error)
	GetByID(ctx context.Context, id string) (*User, error)
	GetByUsername(ctx context.Context, username string) (*User, error)
	Create(ctx context.Context, cmd *CreateCommand) (*User, error)
	Delete(ctx context.Context, provider, providerID string) error
	FindOrCreateByOIDC(ctx context.Context, provider, subject, username string) (string, error)
}

// CreateCommand for creating a user (service layer).
type CreateCommand struct {
	Username   string
	Provider   string
	ProviderID string
}

type service struct {
	repo UserRepository
}

// NewService creates a new user service.
func NewService(repo UserRepository) UserService {
	return &service{
		repo: repo,
	}
}

func (s *service) Get(ctx context.Context, provider, providerID string) (*User, error) {
	user, err := s.repo.GetByProvider(ctx, provider, providerID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.NotFound("user not found")
	}
	return user, nil
}

func (s *service) GetByID(ctx context.Context, id string) (*User, error) {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.NotFound("user not found")
	}
	return user, nil
}

func (s *service) GetByUsername(ctx context.Context, username string) (*User, error) {
	user, err := s.repo.GetByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.NotFound("user not found")
	}
	return user, nil
}

func (s *service) Create(ctx context.Context, cmd *CreateCommand) (*User, error) {
	if cmd.Provider == "" {
		return nil, errors.BadRequest("provider is required")
	}
	if cmd.ProviderID == "" {
		return nil, errors.BadRequest("providerId is required")
	}
	if !IsValidProvider(cmd.Provider) {
		return nil, errors.BadRequest("invalid provider")
	}

	exists, err := s.repo.ExistsByProvider(ctx, cmd.Provider, cmd.ProviderID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.Conflict("user already exists")
	}

	// Generate ID for the user
	userID := uuid.New().String()
	now := time.Now()

	newUser := NewUser(
		userID,
		cmd.Username,
		cmd.Provider,
		cmd.ProviderID,
		now, // joinDate
		now, // createdAt
		now, // updatedAt
	)
	return s.repo.Create(ctx, newUser)
}

func (s *service) Delete(ctx context.Context, provider, providerID string) error {
	user, err := s.repo.GetByProvider(ctx, provider, providerID)
	if err != nil {
		return err
	}
	if user == nil {
		return errors.NotFound("user not found")
	}
	return s.repo.Delete(ctx, user.ID())
}

func (s *service) FindOrCreateByOIDC(ctx context.Context, provider, subject, username string) (string, error) {
	user, err := s.repo.GetByProvider(ctx, provider, subject)
	if err != nil {
		return "", err
	}

	if user != nil {
		return user.ID(), nil
	}

	createCmd := &CreateCommand{
		Username:   username,
		Provider:   provider,
		ProviderID: subject,
	}

	newUser, err := s.Create(ctx, createCmd)
	if err != nil {
		return "", err
	}

	return newUser.ID(), nil
}
