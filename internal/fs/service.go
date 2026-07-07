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

	"github.com/mandacode-labs/mdrive/internal/fs/permission"
)

// Service is the syscall surface for the fs subsystem.
// Mirrors fs/open.c and fs/namei.c in the Linux kernel.
// Result types are narrow: *Dentry is opaque, Stat mirrors
// POSIX struct stat, DirEntry mirrors getdents64; *Node
// never escapes.
type Service interface {
	Walk(ctx context.Context, driveID, path string) (*Dentry, error)
	WalkOne(ctx context.Context, parent *Dentry, name string) (*Dentry, error)
	Create(ctx context.Context, driveID, path string, kind NodeKind) (Stat, error)
	SymlinkAt(ctx context.Context, driveID, target, linkPath string) (Stat, error)
	LinkAt(ctx context.Context, driveID, srcPath, linkPath string) (Stat, error)
	CreateObject(ctx context.Context, driveID, path string, ref ObjectRef, size int64) (Stat, error)
	Unlink(ctx context.Context, driveID, path string) error
	Rmdir(ctx context.Context, driveID, path string) error
	Remove(ctx context.Context, driveID string, paths []string, opts RemoveOpts) error
	RenameAt(ctx context.Context, driveID, srcPath, dstDriveID, dstPath string) error
	Read(ctx context.Context, driveID, path string) ([]byte, error)
	Write(ctx context.Context, driveID, path string, data []byte) error
	ReadlinkAt(ctx context.Context, driveID, path string) (string, error)
	Getdents(ctx context.Context, driveID, path string) ([]DirEntry, error)
	BindMount(ctx context.Context, driveID, mountPath, sourceDriveID string) error
	Unmount(ctx context.Context, driveID, mountPath string) error
	Stat(ctx context.Context, driveID, path string, follow bool) (Stat, error)
}

// RemoveOpts controls Remove behavior.
type RemoveOpts struct {
	Recursive bool
}

// DirEntry is one entry from Getdents. Also the on-disk shape
// stored inside a directory node's data field via DirContent.
type DirEntry struct {
	NodeID uuid.UUID `json:"id"`
	Name   string    `json:"name"`
	Kind   NodeKind  `json:"kind"`
}

// ObjectRef is the public S3 metadata envelope for
// CreateObject.
type ObjectRef struct {
	Bucket   string
	Key      string
	Mime     string
	Checksum string
}

// fs is the concrete Service. The vfs field carries the
// inode layer (concrete type lives in the vfs subpackage).
type fs struct {
	vfs     VFS
	perm    permission.Authorizer
	garbage GarbageRecorder
}

// Config groups the dependencies of New. The caller must
// construct the VFS (typically vvs.New) and pass it in to
// keep the parent fs decoupled from the vfs subpackage.
type Config struct {
	VFS     VFS
	Perm    permission.Authorizer
	Garbage GarbageRecorder
}

// New wires a Service.
func New(cfg Config) Service {
	return &fs{
		vfs:     cfg.VFS,
		perm:    cfg.Perm,
		garbage: cfg.Garbage,
	}
}

var _ Service = (*fs)(nil)
