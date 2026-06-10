package storage

import (
	"context"
	"time"

	"github.com/mandacode-labs/mdrive/internal/core/object"
)

// ObjectService defines the subset of object operations needed by the storage layer.
type ObjectService interface {
	InitiateUpload(ctx context.Context, cmd *object.InitiateUploadCommand) (*object.UploadSession, error)
	CompleteUpload(ctx context.Context, objectID string) (*object.Object, error)
	GetObjectSize(ctx context.Context, id string) (int64, error)
	GetByID(ctx context.Context, id string) (*object.Object, error)
	GetDownloadURL(ctx context.Context, id string, size int64) (string, time.Time, error)
	Delete(ctx context.Context, id string) error
}
