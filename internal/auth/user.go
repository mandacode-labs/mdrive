package auth

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/user"
)

// UserService defines the interface for user operations.
type UserService interface {
	// FindOrCreate finds an existing user by OIDC subject or creates a new one.
	// Returns userID (string).
	FindOrCreate(ctx context.Context, subject, email, name, picture string) (string, error)
}

type userService struct {
	userSvc user.UserService
}

// NewUserService creates a new user adapter.
func NewUserService(userSvc user.UserService) UserService {
	return &userService{
		userSvc: userSvc,
	}
}

// FindOrCreate finds an existing user by OIDC subject or creates a new one.
func (a *userService) FindOrCreate(ctx context.Context, subject, email, name, picture string) (string, error) {
	username := name
	if username == "" {
		username = email
	}
	return a.userSvc.FindOrCreateByOIDC(ctx, "keycloak", subject, username)
}
