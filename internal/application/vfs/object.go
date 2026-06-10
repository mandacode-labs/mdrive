package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/core/object"
)

// ObjectService defines the subset of object operations needed by the VFS layer.
type ObjectService interface {
	CompleteUpload(ctx context.Context, objectID string) (*object.Object, error)
	GetObjectSize(ctx context.Context, id string) (int64, error)
	Delete(ctx context.Context, id string) error
	DeleteBatch(ctx context.Context, ids []string) ([]string, error)
	Find(ctx context.Context, filter object.Filter) ([]*object.Object, error)
}
