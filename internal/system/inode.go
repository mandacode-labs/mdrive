package system

import "context"

// InodeService defines the subset of inode operations needed by the system layer.
type InodeService interface {
	DeleteBySystemID(ctx context.Context, systemID string) error
}
