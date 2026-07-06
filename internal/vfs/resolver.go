package vfs

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"github.com/mandacode-labs/mdrive/internal/auth"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs/content"
	"github.com/mandacode-labs/mdrive/internal/vfs/permission"
	"github.com/oklog/ulid/v2"
)

// Resolver walks a path component-by-component, mirroring Linux
// link_path_walk. The single entry point is Walk; callers that
// need the parent directory compose Walk on the parent path
// themselves (or inspect Dentry.Parent after a successful Walk).
//
// Behavior per component:
//  1. Authorizer.Check(action, currentDriveID) — permission gate.
//  2. NodeOperation.Lookup(currentParent, name) — resolve the
//     child entry.
//  3. If the resolved node is mount-kind: read the source drive
//     id from the inline content, transition to that drive's
//     root, re-check perm.
//  4. If the resolved node is symlink-kind (and we're resolving
//     for read): follow the symlink (depth-capped at 8).
//
// Walk never creates nodes. It only resolves.
type Resolver interface {
	Walk(ctx context.Context, driveID string, path string, action permission.Action) (*Dentry, error)
}

type resolver struct {
	nodeOp  NodeOperation
	driveOp DriveOperation
	perm    permission.Authorizer
}

// NewResolver wires the canonical impl.
func NewResolver(nodeOp NodeOperation, driveOp DriveOperation, perm permission.Authorizer) Resolver {
	return &resolver{
		nodeOp:  nodeOp,
		driveOp: driveOp,
		perm:    perm,
	}
}

var _ Resolver = (*resolver)(nil)

func (r *resolver) userID(ctx context.Context) string {
	return auth.UserIDFromContext(ctx)
}

func (r *resolver) checkPerm(ctx context.Context, action permission.Action, driveID ulid.ULID) error {
	uid := r.userID(ctx)
	ok, err := r.perm.Check(ctx, uid, action, permission.ObjectTypeDrive, driveID.String())
	if err != nil {
		return errorx.Wrap(err, "vfs: permission check failed", errorx.KindUnavailable)
	}
	if !ok {
		return errorx.New(errorx.KindPermissionDenied, "vfs: permission denied")
	}
	return nil
}

func (r *resolver) Walk(ctx context.Context, driveID string, path string, action permission.Action) (*Dentry, error) {
	startDrive, err := parseDriveID(driveID)
	if err != nil {
		return nil, err
	}
	components := splitPath(path)
	root, err := r.rootOf(ctx, startDrive)
	if err != nil {
		return nil, err
	}
	if err := r.checkPerm(ctx, action, startDrive); err != nil {
		return nil, err
	}

	cur := &Dentry{Name: "/", Node: root}
	if len(components) == 0 {
		return cur, nil
	}

	for i, name := range components {
		dentry, err := r.step(ctx, cur, name, action)
		if err != nil {
			return nil, err
		}
		cur = dentry
		_ = i
	}
	return cur, nil
}

// step walks one component under `cur`. The returned Dentry has
// its Parent set to `cur`.
func (r *resolver) step(ctx context.Context, cur *Dentry, name string, action permission.Action) (*Dentry, error) {
	if name == "" || name == "." {
		return cur, nil
	}
	dentry, err := r.nodeOp.Lookup(ctx, cur.Node, name)
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
		if err := r.checkPerm(ctx, permission.ActionView, srcULID); err != nil {
			return nil, err
		}
		root, err := r.rootOf(ctx, srcULID)
		if err != nil {
			return nil, err
		}
		out.Parent = root
		out.Name = "/"
		out.Node = root
	}

	if dentry.Node.Kind() == NodeKindSymlink && action == permission.ActionView {
		resolved, err := r.followSymlink(ctx, out, 8)
		if err != nil {
			return nil, err
		}
		out = resolved
		_ = action
	}
	return out, nil
}

// followSymlink reads the symlink's target id and recurses into
// Lookup from the current drive's root. Depth-capped to prevent
// loops.
func (r *resolver) followSymlink(ctx context.Context, cur *Dentry, depth int) (*Dentry, error) {
	if depth == 0 {
		return nil, errorx.New(errorx.KindFailedPrecondition, "vfs: symlink loop")
	}
	var sc content.SymlinkContent
	if err := json.Unmarshal(cur.Node.Data(), &sc); err != nil {
		return nil, errorx.Wrap(err, "vfs: invalid symlink content")
	}
	drive := cur.Node.Drive()
	root, err := r.rootOf(ctx, drive)
	if err != nil {
		return nil, err
	}
	target, err := r.nodeOp.Lookup(ctx, root, sc.NodeID.String())
	if err != nil {
		return nil, err
	}
	resolved := &Dentry{Parent: root, Name: target.Name, Node: target.Node}
	return r.followSymlink(ctx, resolved, depth-1)
}

// rootOf returns the root inode of a drive. The Drive model
// stores root as a uuid.UUID; we wrap it into a synthetic Node.
func (r *resolver) rootOf(ctx context.Context, driveID ulid.ULID) (*Node, error) {
	d, err := r.driveOp.GetDrive(ctx, driveID.String())
	if err != nil {
		return nil, errorx.Wrap(err, "vfs: failed to load drive", errorx.KindInternal)
	}
	if d == nil {
		return nil, errorx.New(errorx.KindNotFound, "vfs: drive not found")
	}
	return NewNode(d.Root(), driveID, NodeKindDirectory), nil
}

// splitPath splits an absolute path into components. Empty paths
// return an empty slice (root).
func splitPath(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

// silence unused uuid import in this file's non-test builds.
var _ = uuid.Nil
