package dentry

import (
	"context"

	"github.com/mandacode-labs/retrowin-go/internal/core/inode/content"
)

// DirEntry is a type alias for content.DirEntry.
type DirEntry = content.DirEntry

// DentryService defines the interface for directory entry operations.
// Pure data manipulation layer — no permission checks.
type DentryService interface {
	// Link adds a directory entry to a directory inode.
	Link(ctx context.Context, dirID string, entry DirEntry) error
	// Unlink removes a directory entry from a directory inode by name.
	Unlink(ctx context.Context, dirID string, name string) error
	// RenameAt atomically replaces a directory entry, returning the previous inode ID.
	RenameAt(ctx context.Context, dirID string, entry DirEntry) (string, error)
	// ReadDir returns all directory entries for a directory inode.
	ReadDir(ctx context.Context, id string) ([]DirEntry, error)
	// Lookup finds a single directory entry by name.
	Lookup(ctx context.Context, dirID string, name string) (*DirEntry, error)
}
