package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/core/inode"
)

// InodeService defines the subset of inode operations needed by the VFS layer.
type InodeService interface {
	Create(ctx context.Context, cmd *inode.CreateCommand) (*inode.Inode, error)
	GetByID(ctx context.Context, id string) (*inode.Inode, error)
	Update(ctx context.Context, cmd *inode.UpdateCommand) error
	Delete(ctx context.Context, id string) error
	Find(ctx context.Context, filter inode.Filter) ([]*inode.Inode, error)
	Link(ctx context.Context, dirID string, entry inode.DirEntry) error
	Unlink(ctx context.Context, dirID string, name string) error
	UnlinkBatch(ctx context.Context, dirID string, names []string) error
	ReadDir(ctx context.Context, dirID string) ([]inode.DirEntry, error)
	RenameAt(ctx context.Context, dirID string, entry inode.DirEntry) (string, error)
}
