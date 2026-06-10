package system

import "context"

// UserService defines the subset of user operations needed by the system layer.
type UserService interface {
	DeleteBySystemID(ctx context.Context, systemID string) error
}
