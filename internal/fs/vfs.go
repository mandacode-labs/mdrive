// Package fs is mdrive's filesystem layer.
//
// Layering mirrors Linux fs/:
//
//	SYSCALL_DEFINE*(name) → Service method
//	do_*(...)             → fs.doX
//	vfs_*(...)            → fs.vfs.X (vfs subpackage)
//
// Permission checks live on Service; the vfs subpackage
// operates on already-resolved *Dentry.
package fs

import (
	"context"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
)

// VFS is the inode layer (Linux vfs_*). Methods take an
// already-resolved *Dentry. Permission is the caller's job.
type VFS interface {
	lookup(ctx context.Context, driveID ulid.ULID, path string, follow bool) (*Dentry, error)
	walkOne(ctx context.Context, parent *Dentry, name string) (*Dentry, error)
	followMount(ctx context.Context, mount *Node) (*Dentry, error)
	followSymlink(ctx context.Context, cur *Dentry, depth int) (*Dentry, error)
	create(ctx context.Context, parent *Dentry, child *Node, name string) error
	mkdir(ctx context.Context, parent *Dentry, name string) (*Node, error)
	symlink(ctx context.Context, parent *Dentry, name string, targetID uuid.UUID) error
	link(ctx context.Context, oldDentry *Dentry, parent *Dentry, name string) error
	unlink(ctx context.Context, parent *Dentry, name string) error
	rmdir(ctx context.Context, parent *Dentry, name string) error
	rename(ctx context.Context, oldParent *Dentry, oldName string, newParent *Dentry, newName string) error
	read(ctx context.Context, dentry *Dentry) ([]byte, error)
	write(ctx context.Context, dentry *Dentry, data []byte) error
	readlink(ctx context.Context, dentry *Dentry) (uuid.UUID, error)
	getattr(ctx context.Context, dentry *Dentry) (Stat, error)
	iterate(ctx context.Context, parent *Dentry) ([]DirEntry, error)
	mount(ctx context.Context, parent *Dentry, name string, sourceDriveID ulid.ULID) error
	removeRecursive(ctx context.Context, dentry *Dentry) error
}

// GarbageRecorder writes tombstone rows for deleted S3
// objects.
type GarbageRecorder interface {
	RecordGarbage(ctx context.Context, refs []GarbageRef) error
}

// GarbageRef identifies a tombstoned S3 object.
type GarbageRef struct {
	Bucket string
	Key    string
}
