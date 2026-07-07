package fs

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/provider"
)

// VFS is the inode layer (Linux vfs_*). Methods take an
// already-resolved *Dentry. Permission is the caller's job.
type VFS interface {
	Walk(ctx context.Context, driveID ulid.ULID, path string, follow bool) (*Dentry, error)
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
	SetTimes(ctx context.Context, dentry *Dentry) error
	Iterate(ctx context.Context, parent *Dentry) ([]DirEntry, error)
	Mount(ctx context.Context, parent *Dentry, name string, sourceDriveID ulid.ULID) error
	Unmount(ctx context.Context, parent *Dentry, name string) error
	RemoveRecursive(ctx context.Context, dentry *Dentry) error

	// Storage-backed ops (provider-based, no per-drive state).
	Download(ctx context.Context, dentry *Dentry, expiry time.Duration) (string, error)
	Upload(ctx context.Context, parent *Dentry, key, contentType string, expiry time.Duration) (provider.UploadInfo, error)
	Verify(ctx context.Context, dentry *Dentry) (provider.ObjectMetadata, error)
	VerifyByKey(ctx context.Context, key string) (provider.ObjectMetadata, error)
}
