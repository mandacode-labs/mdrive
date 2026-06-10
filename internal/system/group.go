package system

import "context"

// GroupService defines the subset of group operations needed by the system layer.
type GroupService interface {
	DeleteBySystemID(ctx context.Context, systemID string) error
}
