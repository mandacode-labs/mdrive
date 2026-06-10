package sysinit

import (
	"context"

	"github.com/mandacode-labs/retrowin-go/internal/core/inode"
)

// InodeService defines the subset of inode operations needed by sysinit.
type InodeService interface {
	Link(ctx context.Context, dirID string, entry inode.DirEntry) error
}
