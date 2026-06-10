package handler

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/core/inode"
)

// InodeService defines the subset of inode operations needed by the handler layer.
type InodeService interface {
	ReadDir(ctx context.Context, dirID string) ([]inode.DirEntry, error)
}
