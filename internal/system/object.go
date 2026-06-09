package system

import "context"

// ObjectService defines the subset of object operations needed by the system layer.
type ObjectService interface {
	CleanupStorageBySystemID(ctx context.Context, systemID string) error
	DeleteBySystemID(ctx context.Context, systemID string) error
}
