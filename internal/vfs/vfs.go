package vfs

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/vfs/permission"
)

// VFS is the high-level vfs_* helper surface. Mirrors Linux VFS:
// Walk + perm check + dispatch into NodeOperation / DriveOperation.
// syscall.FS passes through to this interface; handlers depend on
// syscall, not vfs directly.
//
// Differences from Linux (deliberate):
//   - No super_operations. Node operations own their tx (in-memory +
//     DB write). Linux separates writeback via super_operations; we don't.
//   - Drive CRUD is mdrive-specific (multi-tenant storage unit).
//   - Garbage collection deferred.
type VFS interface {
	// Path resolution. Linux: link_path_walk.
	Walk(ctx context.Context, driveID string, path string, action permission.Action) (*Dentry, error)

	// Inode creation. Linux: vfs_create + vfs_mkdir + vfs_mknod
	// (unified via kind). Symlink stays separate (Linux vfs_symlink).
	Create(ctx context.Context, driveID string, path string, kind NodeKind, data []byte) (*Node, error)
	Symlink(ctx context.Context, driveID string, target string, linkPath string) error

	// Inode removal. Linux: vfs_unlink + vfs_rmdir (separated).
	Unlink(ctx context.Context, driveID string, path string) error
	Rmdir(ctx context.Context, driveID string, path string) error

	// Inode mutation. Linux: vfs_rename + vfs_link.
	Rename(ctx context.Context, srcDriveID string, srcPath string, dstDriveID string, dstPath string) error
	Link(ctx context.Context, driveID string, srcPath string, linkPath string) error

	// Mount. Linux: vfs_mount.
	Mount(ctx context.Context, driveID string, mountPath string, sourceDriveID string) error

	// Info. Linux: vfs_stat + vfs_lstat + vfs_readlink (separated).
	Stat(ctx context.Context, driveID string, path string) (*Node, error)
	Lstat(ctx context.Context, driveID string, path string) (*Node, error)
	Readlink(ctx context.Context, driveID string, path string) (string, error)

	// Data I/O. Linux: vfs_read + vfs_write.
	Read(ctx context.Context, driveID string, path string) ([]byte, error)
	Write(ctx context.Context, driveID string, path string, data []byte) error

	// Directory listing. Linux: iterate_dir.
	IterateDir(ctx context.Context, driveID string, path string) ([]DirEntry, error)

	// Drive CRUD (mdrive-specific).
	CreateDrive(ctx context.Context, ownerID string, name string, description string, storage *Storage) (*Drive, error)
	GetDrive(ctx context.Context, driveID string) (*Drive, error)
	GetDriveStorage(ctx context.Context, driveID string) (*Storage, error)
	UpdateDrive(ctx context.Context, driveID string, name string, description string) (*Drive, error)
	SoftDeleteDrive(ctx context.Context, driveID string) error
	RestoreDrive(ctx context.Context, driveID string) (*Drive, error)
	PurgeDrive(ctx context.Context, driveID string) error
	ListDrives(ctx context.Context, userID string) ([]*Drive, error)
	ListDeletedDrives(ctx context.Context, before time.Time, limit int) ([]*Drive, error)
}

// DirEntry is one entry from IterateDir (Linux readdir result).
type DirEntry struct {
	InodeID uuid.UUID
	Name    string
	Kind    NodeKind
}

// vfs is the unexported impl of VFS. It owns the Resolver and
// the two operation interfaces; methods dispatch through them.
type vfs struct {
	resolver Resolver
	nodeOp   NodeOperation
	driveOp  DriveOperation
}

// NewVFS wires the canonical impl.
func NewVFS(resolver Resolver, nodeOp NodeOperation, driveOp DriveOperation) VFS {
	return &vfs{
		resolver: resolver,
		nodeOp:   nodeOp,
		driveOp:  driveOp,
	}
}

var _ VFS = (*vfs)(nil)

// silence unused-import linter when only used in tests.
var _ = time.Time{}
