package vfs

import (
	"context"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
)

// SuperOperation is the minimal contract vfs needs to access
// the root inode of a drive. Linux's super_operations interface
// is much broader (alloc_inode, write_inode, drop_inode, ...),
// but for path resolution we only need the root lookup.
//
// Implemented by the superblock package; vfs itself only
// depends on this interface, not on the concrete package, to
// avoid a circular dependency between vfs and superblock.
type SuperOperation interface {
	GetRootNodeID(ctx context.Context, driveID ulid.ULID) (uuid.UUID, error)
}