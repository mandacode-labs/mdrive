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
// It mirrors the Linux VFS layer: path → walk → dentry/inode.
// Path resolution is private — callers pass paths, vfs runs
// walk internally (see walk.go).
type VFS interface {
	// Create an empty inode (Linux vfs_create + vfs_mkdir + vfs_mknod,
	// unified by NodeKind). Kind-specific data is set by Write /
	// WriteObject / Mount / Symlink.
	Create(ctx context.Context, driveID string, path string, kind NodeKind) (*Node, error)

	// Symlink creates a link at linkPath pointing at target.
	// Target id is stored inline via content.SymlinkContent.
	Symlink(ctx context.Context, driveID string, target string, linkPath string) error

	// Mount installs a mount point at mountPath into sourceDriveID's
	// root. Mount kind stores source drive id inline as
	// content.MountContent.
	Mount(ctx context.Context, driveID string, mountPath string, sourceDriveID string) error

	// Unlink removes a non-directory. Rmdir removes an empty
	// directory. nlink==0 destroys the inode. Linux vfs_unlink + vfs_rmdir.
	Unlink(ctx context.Context, driveID string, path string) error
	Rmdir(ctx context.Context, driveID string, path string) error

	// Rename moves an entry. Cross-drive rename is not supported.
	// Link adds a hard link. Linux vfs_rename + vfs_link.
	Rename(ctx context.Context, srcDriveID string, srcPath string, dstDriveID string, dstPath string) error
	Link(ctx context.Context, driveID string, srcPath string, linkPath string) error

	// Stat follows the trailing symlink. Lstat returns the symlink
	// itself. Readlink returns the symlink target's inode id
	// (graph-based, not raw path).
	Stat(ctx context.Context, driveID string, path string) (*Node, error)
	Lstat(ctx context.Context, driveID string, path string) (*Node, error)
	Readlink(ctx context.Context, driveID string, path string) (uuid.UUID, error)

	// Read returns the inline data of a file-kind node. Write
	// overwrites it. Object-kind reads use ReadObject; writes use
	// WriteObject.
	Read(ctx context.Context, driveID string, path string) ([]byte, error)
	Write(ctx context.Context, driveID string, path string, data []byte) error

	// WriteObject stores S3 metadata of an Object-kind node from a
	// completed upload. ReadObject retrieves it for the caller
	// (typically upload.PresignDownload). vfs carries no S3 client.
	WriteObject(ctx context.Context, driveID string, path string, ref ObjectRef) error
	ReadObject(ctx context.Context, driveID string, path string) (ObjectRef, error)

	// IterateDir returns the direct children of a directory-kind
	// node (Linux iterate_dir / getdents64).
	IterateDir(ctx context.Context, driveID string, path string) ([]DirEntry, error)
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

// userID extracts the caller's user id from the request context.
func (v *vfs) userID(ctx context.Context) string {
	return auth.UserIDFromContext(ctx)
}

// walkEntry pairs the walk result with the starting superblock
// so callers can mint new inodes bound to the same sb without
// a separate superop round-trip.
func (v *vfs) walkEntry(ctx context.Context, driveID ulid.ULID, path string, follow bool) (*Dentry, *Superblock, error) {
	sb, err := v.superop.GetByDriveID(ctx, driveID)
	if err != nil {
		return nil, nil, errorx.Wrap(err, "vfs: failed to get superblock", errorx.KindInternal)
	}
	dentry, err := v.walkFrom(ctx, sb, path, follow)
	if err != nil {
		return nil, nil, err
	}
	return dentry, sb, nil
}

// loadSB returns the superblock backing a drive id.
func (v *vfs) loadSB(ctx context.Context, driveID ulid.ULID) (*Superblock, error) {
	return v.superop.GetByDriveID(ctx, driveID)
}

// walkEntryNoFollow / walkEntryNoFollowErr aliases folded into
// walkEntry by parameter; left intentionally absent to keep the
// single call form.

// walkFrom resolves `path` from the given superblock. Used by
// walkEntry and as the core path-walking primitive.
func (v *vfs) walkFrom(ctx context.Context, sb *Superblock, path string, follow bool) (*Dentry, error) {
	if err := v.checkPerm(ctx, permission.ActionView, sb.DriveID()); err != nil {
		return nil, err
	}
	root, err := v.nodeOp.Get(ctx, sb.RootNodeID())
	if err != nil {
		return nil, errorx.Wrap(err, "vfs: failed to get root node", errorx.KindInternal)
	}
	if root.Kind() != NodeKindDirectory {
		return nil, errorx.New(errorx.KindInternal, "vfs: drive root is not a directory")
	}

	cur := &Dentry{Parent: nil, Name: "/", Node: root}
	for _, name := range splitPath(path) {
		next, err := v.walkStep(ctx, cur, name)
		if err != nil {
			return nil, err
		}
		cur = next
	}

	if follow && cur.Node.Kind() == NodeKindSymlink {
		return v.followSymlink(ctx, cur, 8)
	}
	return cur, nil
}

// checkPerm gates a drive-level permission. walk invokes it
// once on the starting drive (ActionView) and again on each
// mount boundary; mutation commands layer their own check on top.
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
