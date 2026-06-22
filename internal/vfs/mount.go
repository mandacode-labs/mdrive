package vfs

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/permission"
)

// Mount creates a bind mount of sourceDriveID at mountPath inside
// driveID's tree. The mount is a directory entry; resolving through it
// switches context to the source drive and continues with the remaining
// path. Permissions: caller must have edit on driveID (to create the
// mount entry) and view on sourceDriveID (to verify the source exists
// and is accessible).
//
// Same-drive mounts (driveID == sourceDriveID) are rejected to avoid
// trivial self-cycles.
func (s *Service) Mount(ctx context.Context, userID, driveID, mountPath, sourceDriveID string) (*node.Node, error) {
	if driveID == sourceDriveID {
		return nil, fmt.Errorf("mount: cannot mount a drive onto itself")
	}
	if err := s.checkAccess(ctx, userID, permission.PermissionEdit, driveID); err != nil {
		return nil, err
	}
	if err := s.checkAccess(ctx, userID, permission.PermissionView, sourceDriveID); err != nil {
		return nil, fmt.Errorf("mount: source drive: %w", err)
	}
	// Validate source drive exists and has a root.
	src, err := s.Drive.GetByID(ctx, sourceDriveID)
	if err != nil {
		return nil, fmt.Errorf("mount: source drive lookup: %w", err)
	}
	if src == nil || src.RootNodeID() == nil {
		return nil, fmt.Errorf("mount: source drive has no root node")
	}

	rootID, err := s.rootNodeID(ctx, driveID)
	if err != nil {
		return nil, err
	}
	parent, name, err := s.path.resolveParent(ctx, rootID, mountPath)
	if err != nil {
		return nil, fmt.Errorf("mount: resolve target: %w", err)
	}
	if !parent.IsDir() {
		return nil, ErrNotDirectory
	}
	mount, err := s.Node.CreateMount(ctx, sourceDriveID)
	if err != nil {
		return nil, err
	}
	if err := s.Node.Link(ctx, parent, name, mount); err != nil {
		_ = s.Node.Delete(ctx, mount.ID())
		return nil, fmt.Errorf("mount: link: %w", err)
	}
	return mount, nil
}

// Unmount removes the mount at mountPath within driveID. The mount
// node is deleted; the source drive and its data are untouched.
// Permissions: caller must have edit on driveID.
//
// If the entry at mountPath is not a mount, returns an error.
func (s *Service) Unmount(ctx context.Context, userID, driveID, mountPath string) error {
	if err := s.checkAccess(ctx, userID, permission.PermissionEdit, driveID); err != nil {
		return err
	}
	rootID, err := s.rootNodeID(ctx, driveID)
	if err != nil {
		return err
	}
	n, err := s.path.resolve(ctx, rootID, mountPath)
	if err != nil {
		return fmt.Errorf("unmount: %w", err)
	}
	if !n.IsMount() {
		return fmt.Errorf("unmount: %s is not a mount", mountPath)
	}
	parent, name, err := s.path.resolveParent(ctx, rootID, mountPath)
	if err != nil {
		return fmt.Errorf("unmount: resolve parent: %w", err)
	}
	if _, err := s.Node.Unlink(ctx, parent, name); err != nil {
		return fmt.Errorf("unmount: unlink: %w", err)
	}
	return nil
}

// resolveCrossDrive walks a path that may cross mount nodes. The resolver
// keeps a visited set of drive ids to prevent mount cycles, and a hop
// counter to bound the traversal depth.
//
// The returned Resolved identifies the drive id and node that the path
// ultimately resolves to.
func (s *Service) resolveCrossDrive(ctx context.Context, driveID, path string) (driveIDOut string, rootOut uuid.UUID, n *node.Node, err error) {
	const maxMountHops = 32
	visited := map[string]struct{}{driveID: {}}
	curDriveID := driveID

	for hop := 0; hop <= maxMountHops; hop++ {
		drive, err := s.Drive.GetByID(ctx, curDriveID)
		if err != nil {
			return "", uuid.Nil, nil, fmt.Errorf("resolve: drive lookup: %w", err)
		}
		if drive == nil || drive.RootNodeID() == nil {
			return "", uuid.Nil, nil, ErrNotFound
		}
		curRootID := *drive.RootNodeID()
		// Resolve the path within the current drive. The resolver follows
		// '..' and '.' but treats mount nodes as ordinary entries (it
		// returns the mount node itself, not the target). The for-loop
		// handles follow-through below.
		current, rest, err := s.resolveWithinDrive(ctx, curRootID, path, curDriveID, visited, maxMountHops-hop)
		if err != nil {
			return "", uuid.Nil, nil, err
		}
		if !current.IsMount() {
			return curDriveID, curRootID, current, nil
		}
		// Mount node: switch context to the source drive. The remaining
		// path is what was left after the mount entry name in the
		// parent's DirContent lookup. If the mount was the last component
		// of the user's path, we return the mount node itself.
		if rest == "" {
			return curDriveID, curRootID, current, nil
		}
		srcDriveID, err := current.ReadMount()
		if err != nil {
			return "", uuid.Nil, nil, fmt.Errorf("resolve: read mount: %w", err)
		}
		if _, seen := visited[srcDriveID]; seen {
			return "", uuid.Nil, nil, fmt.Errorf("resolve: mount cycle detected at drive %s", srcDriveID)
		}
		visited[srcDriveID] = struct{}{}
		curDriveID = srcDriveID
		path = rest
	}
	return "", uuid.Nil, nil, fmt.Errorf("resolve: max mount hops exceeded")
}

// resolveWithinDrive walks a path inside a single drive's tree. When it
// encounters a mount node and the caller is not the root resolver (we
// recurse internally to follow mounts), it returns the mount node
// together with the remaining path so the caller can decide what to do.
func (s *Service) resolveWithinDrive(ctx context.Context, rootID uuid.UUID, p, driveID string, visited map[string]struct{}, remainingHops int) (*node.Node, string, error) {
	cleaned := cleanPath(p)
	if cleaned == "" || cleaned == "/" {
		n, err := s.Node.GetByID(ctx, rootID)
		return n, "", err
	}
	ancestors := make([]*node.Node, 0, 4)
	root, err := s.Node.GetByID(ctx, rootID)
	if err != nil {
		return nil, "", ErrNotFound
	}
	current := root
	ancestors = append(ancestors, current)
	parts := splitPath(cleaned)
	for i, part := range parts {
		switch part {
		case "", ".":
			continue
		case "..":
			if len(ancestors) <= 1 {
				return nil, "", ErrInvalidPath
			}
			ancestors = ancestors[:len(ancestors)-1]
			current = ancestors[len(ancestors)-1]
			continue
		}
		entry, err := current.Lookup(part)
		if err != nil {
			return nil, "", err
		}
		if entry == nil {
			return nil, "", ErrNotFound
		}
		child, err := s.Node.GetByID(ctx, entry.InodeID)
		if err != nil {
			return nil, "", ErrNotFound
		}
		// If this is the last path component, return it directly.
		if i == len(parts)-1 {
			return child, "", nil
		}
		// Otherwise, if it is a mount, follow it before continuing.
		if child.IsMount() {
			srcDriveID, err := child.ReadMount()
			if err != nil {
				return nil, "", fmt.Errorf("resolve: read mount: %w", err)
			}
			if _, seen := visited[srcDriveID]; seen {
				return nil, "", fmt.Errorf("resolve: mount cycle detected at drive %s", srcDriveID)
			}
			if remainingHops <= 0 {
				return nil, "", fmt.Errorf("resolve: max mount hops exceeded")
			}
			srcDrive, err := s.Drive.GetByID(ctx, srcDriveID)
			if err != nil || srcDrive == nil || srcDrive.RootNodeID() == nil {
				return nil, "", ErrNotFound
			}
			visited[srcDriveID] = struct{}{}
			rest := "/" + strings.Join(parts[i+1:], "/")
			_, _, n, err := s.resolveCrossDriveInner(ctx, srcDriveID, *srcDrive.RootNodeID(), rest, visited, remainingHops-1)
			return n, "", err
		}
		ancestors = append(ancestors, child)
		current = child
	}
	return current, "", nil
}

// resolveCrossDriveInner is the inner loop extracted to allow
// resolveWithinDrive to recurse into mounts without re-allocating the
// visited set.
func (s *Service) resolveCrossDriveInner(ctx context.Context, driveID string, rootID uuid.UUID, path string, visited map[string]struct{}, remainingHops int) (string, uuid.UUID, *node.Node, error) {
	curDriveID := driveID
	curRootID := rootID
	currentPath := path
	for hop := 0; hop <= remainingHops; hop++ {
		current, _, err := s.resolveWithinDrive(ctx, curRootID, currentPath, curDriveID, visited, remainingHops-hop)
		if err != nil {
			return "", uuid.Nil, nil, err
		}
		if !current.IsMount() {
			return curDriveID, curRootID, current, nil
		}
		srcDriveID, err := current.ReadMount()
		if err != nil {
			return "", uuid.Nil, nil, fmt.Errorf("resolve: read mount: %w", err)
		}
		if _, seen := visited[srcDriveID]; seen {
			return "", uuid.Nil, nil, fmt.Errorf("resolve: mount cycle detected at drive %s", srcDriveID)
		}
		visited[srcDriveID] = struct{}{}
		srcDrive, err := s.Drive.GetByID(ctx, srcDriveID)
		if err != nil || srcDrive == nil || srcDrive.RootNodeID() == nil {
			return "", uuid.Nil, nil, ErrNotFound
		}
		curDriveID = srcDriveID
		curRootID = *srcDrive.RootNodeID()
		currentPath = "/"
	}
	return "", uuid.Nil, nil, fmt.Errorf("resolve: max mount hops exceeded")
}
