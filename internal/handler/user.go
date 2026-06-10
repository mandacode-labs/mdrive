package handler

import (
	"context"

	coreuser "github.com/mandacode-labs/mdrive/internal/core/user"
)

// UserService defines the subset of user operations needed by the handler layer.
type UserService interface {
	GetByUserAndSystem(ctx context.Context, userID, systemID string) (*coreuser.SystemUser, error)
	Create(ctx context.Context, cmd *coreuser.CreateCommand) (*coreuser.SystemUser, error)
	Find(ctx context.Context, filter coreuser.Filter) ([]*coreuser.SystemUser, error)
	FindOne(ctx context.Context, filter coreuser.Filter) (*coreuser.SystemUser, error)
	Delete(ctx context.Context, id int) error
}
