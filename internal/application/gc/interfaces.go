package gc

import (
	"context"
	"time"

	"github.com/mandacode-labs/retrowin-go/internal/core/object"
)

// ObjectService defines the subset of object operations needed by the garbage collector.
type ObjectService interface {
	FindPendingOlderThan(ctx context.Context, olderThan time.Duration) ([]*object.Object, error)
	FindActive(ctx context.Context) ([]*object.Object, error)
	Delete(ctx context.Context, id string) error
	DeleteFromDB(ctx context.Context, id string) error
}
