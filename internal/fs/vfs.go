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
	Lookup(ctx context.Context, driveID ulid.ULID, path string, follow bool) (*Dentry, error)
	WalkOne(ctx context.Context, parent *Dentry, name string) (*Dentry, error)
	FollowMount(ctx context.Context, parent *Dentry, mount *Node) (*Dentry, error)
	FollowSymlink(ctx context.Context, cur *Dentry, depth int) (*Dentry, error)
	Create(ctx context.Context, parent *Dentry, child *Node, name string) error
	Mkdir(ctx context.Context, parent *Dentry, name string) (*Node, error)
	Symlink(ctx context.Context, parent *Dentry, name string, targetID uuid.UUID) (*Node, error)
	Link(ctx context.Context, oldDentry *Dentry, parent *Dentry, name string) error
	Unlink(ctx context.Context, parent *Dentry, name string) error
	Rmdir(ctx context.Context, parent *Dentry, name string) error
	Rename(ctx context.Context, oldParent *Dentry, oldName string, newParent *Dentry, newName string) error
	Read(ctx context.Context, dentry *Dentry) ([]byte, error)
	Write(ctx context.Context, dentry *Dentry, data []byte) error
	Readlink(ctx context.Context, dentry *Dentry) (uuid.UUID, error)
	Getattr(ctx context.Context, dentry *Dentry) (Stat, error)
	Iterate(ctx context.Context, parent *Dentry) ([]DirEntry, error)
	Mount(ctx context.Context, parent *Dentry, name string, sourceDriveID ulid.ULID) error
	RemoveRecursive(ctx context.Context, dentry *Dentry) error
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
