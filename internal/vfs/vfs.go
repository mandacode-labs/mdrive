package vfs

import (
	"context"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/auth"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs/permission"
)

// VFS is the canonical virtual filesystem surface for mdrive.
//
// Layering mirrors Linux: Walk does path lookup (namei /
// path_lookupat); other methods assume the caller has
// resolved the entry and act on *Dentry (and a leaf name for
// mutations). Permission is the caller's job.
type VFS interface {
	// Walk resolves a path into a Dentry. follow selects
	// trailing symlink semantics (true → stat, false → lstat).
	Walk(ctx context.Context, driveID string, path string, follow bool) (*Dentry, error)

	// Inode creation. Caller passes a fresh *Node whose
	// kind-specific data is already set.
	Create(ctx context.Context, parent *Dentry, child *Node, name string) error
	Symlink(ctx context.Context, linkParent *Dentry, linkName string, targetID uuid.UUID) error
	Mount(ctx context.Context, mountParent *Dentry, mountName string, sourceDriveID ulid.ULID) error

	// Inode removal. Linux vfs_unlink + vfs_rmdir. Unlink
	// refuses directories; Rmdir refuses non-empty entries
	// (use Remove with Recursive=true to clear first).
	Unlink(ctx context.Context, parent *Dentry, name string) error
	Rmdir(ctx context.Context, parent *Dentry, name string) error

	// Linux vfs_rename + vfs_link.
	Rename(ctx context.Context, oldParent *Dentry, oldName string, newParent *Dentry, newName string) error
	Link(ctx context.Context, oldDentry *Dentry, linkParent *Dentry, linkName string) error

	// Linux stat(2) / lstat(2) / readlink(2). Readlink returns
	// the symlink target's inode id (mdrive is graph-based).
	Stat(ctx context.Context, dentry *Dentry) (*Node, error)
	Lstat(ctx context.Context, dentry *Dentry) (*Node, error)
	Readlink(ctx context.Context, dentry *Dentry) (uuid.UUID, error)

	// Linux vfs_read + vfs_write on a file-kind node.
	Read(ctx context.Context, dentry *Dentry) ([]byte, error)
	Write(ctx context.Context, dentry *Dentry, data []byte) error

	// Object-kind storage: WriteObject stores S3 metadata from
	// a completed upload; ReadObject retrieves it (caller uses
	// upload.PresignDownload). vfs carries no S3 client.
	WriteObject(ctx context.Context, parent *Dentry, child *Node, name string) error
	ReadObject(ctx context.Context, dentry *Dentry) (ObjectRef, error)

	// Linux iterate_dir / getdents64.
	IterateDir(ctx context.Context, parent *Dentry) ([]DirEntry, error)

	// Remove is mdrive's `rm -rf` equivalent — recursive
	// cascade not provided by in-kernel rmdir/unlink.
	Remove(ctx context.Context, parent *Dentry, name string, opts RemoveOpts) error
}

// RemoveOpts controls Remove behavior.
// Currently only Recursive is exposed; force / error-swallowing
// options are not.
type RemoveOpts struct {
	Recursive bool
}

// DirEntry is one entry from IterateDir.
type DirEntry struct {
	InodeID uuid.UUID
	Name    string
	Kind    NodeKind
}

// ObjectRef is the public S3 metadata envelope for WriteObject /
// ReadObject. Bytes are stored inline as content.ObjectContent;
// the caller owns the actual object storage interaction.
type ObjectRef struct {
	Bucket   string
	Key      string
	Mime     string
	Checksum string
}

type vfs struct {
	nodeOp  NodeOperation
	superop SuperOperation
	perm    permission.Authorizer
}

// NewVFS wires the canonical impl.
func NewVFS(nodeOp NodeOperation, superop SuperOperation, perm permission.Authorizer) VFS {
	return &vfs{
		nodeOp:  nodeOp,
		superop: superop,
		perm:    perm,
	}
}

// Walk implements [VFS]. Mirrors Linux's path lookup entry:
// namei / path_lookupat. Mounts are followed transparently;
// a final symlink is followed only when follow=true.
func (v *vfs) Walk(ctx context.Context, driveID string, path string, follow bool) (*Dentry, error) {
	id, err := ulid.Parse(driveID)
	if err != nil {
		return nil, errorx.Wrap(err, "vfs: invalid drive id", errorx.KindInvalidArgument)
	}
	return v.walk(ctx, id, path, follow)
}

// userID extracts the caller's user id from the request context.
func (v *vfs) userID(ctx context.Context) string {
	return auth.UserIDFromContext(ctx)
}

// checkPerm gates a drive-level permission. walk uses
// ActionView; mutation commands layer ActionEdit on top.
func (v *vfs) checkPerm(ctx context.Context, action permission.Action, driveID ulid.ULID) error {
	uid := v.userID(ctx)
	ok, err := v.perm.Check(ctx, uid, action, permission.ObjectTypeDrive, driveID.String())
	if err != nil {
		return errorx.Wrap(err, "vfs: permission check failed", errorx.KindUnavailable)
	}
	if !ok {
		return errorx.New(errorx.KindPermissionDenied, "vfs: permission denied")
	}
	return nil
}
