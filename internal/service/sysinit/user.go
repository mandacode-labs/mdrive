package sysinit

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/core/user"
)

// UserService defines the subset of user operations needed by sysinit.
type UserService interface {
	Create(ctx context.Context, cmd *user.CreateCommand) (*user.SystemUser, error)
}
