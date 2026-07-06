package vfs

import (
	"context"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/auth"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs/permission"
)

// VFS is the canonical virtual filesystem interface. It is
type VFS interface {
	// Create an empty inode. Linux vfs_create + vfs_mkdir +
	Create(ctx context.Context, driveID string, path string, kind NodeKind) (*Node, error)

	// Create a Symbolic Link. Linux: vfs_symlink. The target is stored inline via content.SymlinkContent.
	Symlink(ctx context.Context, driveID string, target string, linkPath string) error

	// Mount: sourceDriveID is the drive the mount point
	// resolves into. vfs stores the source id inline via
	// content.MountContent.
	Mount(ctx context.Context, driveID string, mountPath string, sourceDriveID string) error

	// Inode removal. Linux: vfs_unlink + vfs_rmdir (separated).
	Unlink(ctx context.Context, driveID string, path string) error
	Rmdir(ctx context.Context, driveID string, path string) error

	// Inode mutation. Linux: vfs_rename + vfs_link.
	Rename(ctx context.Context, srcDriveID string, srcPath string, dstDriveID string, dstPath string) error
	Link(ctx context.Context, driveID string, srcPath string, linkPath string) error

	// Info. Linux: vfs_stat + vfs_lstat + vfs_readlink.
	Stat(ctx context.Context, driveID string, path string) (*Node, error)
	Lstat(ctx context.Context, driveID string, path string) (*Node, error)
	Readlink(ctx context.Context, driveID string, path string) (uuid.UUID, error)

	// Data I/O. Linux: vfs_read + vfs_write. Write overwrites
	// a file-kind node's inline data.
	Read(ctx context.Context, driveID string, path string) ([]byte, error)
	Write(ctx context.Context, driveID string, path string, data []byte) error

	// WriteObject creates or replaces an Object-kind node.
	// ref holds the S3/MinIO location and metadata; vfs stores
	// it inline via content.ObjectContent. Caller (handler)
	// typically invokes this after a successful S3 upload.
	WriteObject(ctx context.Context, driveID string, path string, ref ObjectRef) error

	// ReadObject returns the S3 metadata stored in an Object-kind
	// node. The caller uses the returned ref to download the
	// actual bytes from object storage (e.g. via upload.PresignDownload).
	// Symlinks along the path are followed; the final node must be
	// of Object kind.
	ReadObject(ctx context.Context, driveID string, path string) (ObjectRef, error)

	// Directory listing. Linux: iterate_dir.
	IterateDir(ctx context.Context, driveID string, path string) ([]DirEntry, error)
}

// DirEntry is one entry from IterateDir (Linux readdir result).
type DirEntry struct {
	InodeID uuid.UUID
	Name    string
	Kind    NodeKind
}

// ObjectRef is the public input for WriteObject. The caller
type ObjectRef struct {
	Bucket   string
	Key      string
	Mime     string
	Checksum string
}

// vfs is the unexported impl.
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

// userID is the caller's user id from the request context.
func (v *vfs) userID(ctx context.Context) string {
	return auth.UserIDFromContext(ctx)
}

// checkPerm is the entry-level permission gate.
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
