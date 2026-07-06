package vfs

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/auth"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs/content"
	"github.com/mandacode-labs/mdrive/internal/vfs/permission"
)

// VFS is the high-level vfs_* helper surface. Mirrors Linux VFS:
// each command takes a path and resolves internally (perm
// check + mount crossing + symlink follow) before dispatching
// into NodeOperation / DriveOperation.
//
// syscall.FS passes through to this interface; handlers depend
// on syscall, not vfs directly.
//
// Differences from Linux (deliberate):
//   - No super_operations. Node operations own their tx (in-memory +
//     DB write). Linux separates writeback via super_operations; we don't.
//   - Drive CRUD is mdrive-specific (multi-tenant storage unit).
//   - Garbage collection deferred.
//   - Walk is not exposed here. It runs inside each command's
//     resolveTarget / resolveParent. Equivalent to Linux
//     path_walk being a static function, not a syscall.
type VFS interface {
	// Inode creation. Linux: vfs_create + vfs_mkdir + vfs_mknod
	// (unified via kind). Symlink stays separate (vfs_symlink).
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

// vfs is the unexported impl. It holds the two operation
// interfaces plus the permission authorizer. Walk (resolveTarget
// / resolveParent) is internal — callers pass a path and each
// command resolves it.
type vfs struct {
	nodeOp  NodeOperation
	driveOp DriveOperation
	perm    permission.Authorizer
}

// NewVFS wires the canonical impl.
func NewVFS(nodeOp NodeOperation, driveOp DriveOperation, perm permission.Authorizer) VFS {
	return &vfs{
		nodeOp:  nodeOp,
		driveOp: driveOp,
		perm:    perm,
	}
}

var _ VFS = (*vfs)(nil)

// userID is the caller's user id from the request context.
func (v *vfs) userID(ctx context.Context) string {
	return auth.UserIDFromContext(ctx)
}

// checkPerm is the entry-level permission gate. vfs helpers call
// it after resolveTarget has already done per-drive checks
// during the walk. The redundant-by-design double check makes
// the entry-level intent explicit.
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

// rootOf returns the root inode of a drive.
func (v *vfs) rootOf(ctx context.Context, driveID ulid.ULID) (*Node, error) {
	d, err := v.driveOp.GetDrive(ctx, driveID.String())
	if err != nil {
		return nil, errorx.Wrap(err, "vfs: failed to load drive", errorx.KindInternal)
	}
	if d == nil {
		return nil, errorx.New(errorx.KindNotFound, "vfs: drive not found")
	}
	return NewNode(d.Root(), driveID, NodeKindDirectory), nil
}

// resolveTarget walks `path` from `driveID` and returns the final
// Dentry. Mirrors Linux link_path_walk: per-component perm
// check, mount crossing, symlink follow (depth-capped at 8).
func (v *vfs) resolveTarget(ctx context.Context, driveID string, path string, action permission.Action) (*Dentry, error) {
	startDrive, err := ulid.Parse(driveID)
	if err != nil {
		return nil, errorx.Wrap(err, "vfs: invalid drive id", errorx.KindInvalidArgument)
	}
	components := splitPath(path)
	root, err := v.rootOf(ctx, startDrive)
	if err != nil {
		return nil, err
	}
	if err := v.checkPerm(ctx, action, startDrive); err != nil {
		return nil, err
	}

	cur := &Dentry{Name: "/", Node: root}
	for _, name := range components {
		cur, err = v.step(ctx, cur, name, action)
		if err != nil {
			return nil, err
		}
	}
	return cur, nil
}

// resolveParent walks to the parent directory of `path`.
// Returns the parent Dentry plus the basename.
func (v *vfs) resolveParent(ctx context.Context, driveID string, path string, action permission.Action) (*Dentry, string, error) {
	components := splitPath(path)
	if len(components) == 0 {
		return nil, "", errorx.New(errorx.KindInvalidArgument, "vfs: empty path has no parent")
	}
	name := components[len(components)-1]
	parent, err := v.resolveTarget(ctx, driveID, joinComponents(components[:len(components)-1]), action)
	if err != nil {
		return nil, "", err
	}
	return parent, name, nil
}

// step walks one component under `cur`.
func (v *vfs) step(ctx context.Context, cur *Dentry, name string, action permission.Action) (*Dentry, error) {
	if name == "" || name == "." {
		return cur, nil
	}
	dentry, err := v.nodeOp.Lookup(ctx, cur.Node, name)
	if err != nil {
		return nil, err
	}
	out := &Dentry{Parent: cur.Node, Name: name, Node: dentry.Node}

	if dentry.Node.Kind() == NodeKindMount {
		var mc content.MountContent
		if err := json.Unmarshal(dentry.Node.Data(), &mc); err != nil {
			return nil, errorx.Wrap(err, "vfs: invalid mount content")
		}
		if mc.DriveID == "" {
			return nil, errorx.New(errorx.KindInternal, "vfs: mount without source drive id")
		}
		srcULID, err := ulid.Parse(mc.DriveID)
		if err != nil {
			return nil, errorx.Wrap(err, "vfs: invalid mount source drive id", errorx.KindInternal)
		}
		if err := v.checkPerm(ctx, permission.ActionView, srcULID); err != nil {
			return nil, err
		}
		root, err := v.rootOf(ctx, srcULID)
		if err != nil {
			return nil, err
		}
		out.Parent = root
		out.Name = "/"
		out.Node = root
	}

	if dentry.Node.Kind() == NodeKindSymlink && action == permission.ActionView {
		out, err = v.followSymlink(ctx, out, 8)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// followSymlink reads the symlink's target id and recurses into
// Lookup from the current drive's root. Depth-capped.
func (v *vfs) followSymlink(ctx context.Context, cur *Dentry, depth int) (*Dentry, error) {
	if depth == 0 {
		return nil, errorx.New(errorx.KindFailedPrecondition, "vfs: symlink loop")
	}
	var sc content.SymlinkContent
	if err := json.Unmarshal(cur.Node.Data(), &sc); err != nil {
		return nil, errorx.Wrap(err, "vfs: invalid symlink content")
	}
	drive := cur.Node.Drive()
	root, err := v.rootOf(ctx, drive)
	if err != nil {
		return nil, err
	}
	target, err := v.nodeOp.Lookup(ctx, root, sc.NodeID.String())
	if err != nil {
		return nil, err
	}
	resolved := &Dentry{Parent: root, Name: target.Name, Node: target.Node}
	return v.followSymlink(ctx, resolved, depth-1)
}

// splitPath splits an absolute path into components.
func splitPath(p string) []string {
	if p == "" {
		return nil
	}
	if len(p) > 0 && p[0] == '/' {
		p = p[1:]
	}
	if p == "" {
		return nil
	}
	out := []string{}
	cur := ""
	for _, c := range p {
		if c == '/' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			continue
		}
		cur += string(c)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

// joinComponents joins a component slice back into a path.
func joinComponents(components []string) string {
	if len(components) == 0 {
		return "/"
	}
	out := ""
	for _, c := range components {
		out += "/" + c
	}
	return out
}

// silence unused imports in this file.
var _ = strings.TrimSpace
var _ time.Time
