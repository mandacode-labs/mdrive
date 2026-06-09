package inode

import (
	"context"

	"github.com/mandacode-labs/retrowin-go/internal/core/inode/content"
)

// DirEntry represents a directory entry (filename to inode mapping).
type DirEntry = content.DirEntry

// InodeOperations defines the interface for inode operations.
// This corresponds to Linux's inode_operations and file_operations for directories.
type InodeOperations interface {
	// --- Metadata operations (Linux: inode attributes) ---
	Create(ctx context.Context, cmd *CreateCommand) (*Inode, error)
	GetByID(ctx context.Context, id string) (*Inode, error)
	Update(ctx context.Context, cmd *UpdateCommand) error
	Delete(ctx context.Context, id string) error
	DeleteBySystemID(ctx context.Context, systemID string) error
	Find(ctx context.Context, filter Filter) ([]*Inode, error)
	FindOne(ctx context.Context, filter Filter) (*Inode, error)
	UpdateLinkCount(ctx context.Context, id string, delta int) error

	// --- Directory operations (Linux: inode_operations) ---
	// Link adds a directory entry (inode_operations->link).
	Link(ctx context.Context, dirID string, entry DirEntry) error
	// Unlink removes a directory entry (inode_operations->unlink).
	Unlink(ctx context.Context, dirID string, name string) error
	// UnlinkBatch removes multiple entries atomically.
	UnlinkBatch(ctx context.Context, dirID string, names []string) error
	// RenameAt replaces a directory entry (inode_operations->rename).
	RenameAt(ctx context.Context, dirID string, entry DirEntry) (string, error)
	// ReadDir returns all directory entries (file_operations->readdir).
	ReadDir(ctx context.Context, id string) ([]DirEntry, error)
	// Lookup finds an entry by name (inode_operations->lookup).
	Lookup(ctx context.Context, dirID string, name string) (*DirEntry, error)
}
