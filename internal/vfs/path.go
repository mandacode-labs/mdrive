package vfs

import (
	"context"
	"path"
	"strings"

	"github.com/google/uuid"
	"github.com/mandacode-labs/mdrive/internal/core/node"
)

// resolver walks the node tree from a drive root to resolve Unix paths.
type resolver struct {
	node NodeClient
}

func newResolver(n NodeClient) *resolver {
	return &resolver{node: n}
}

// resolveOutcome is the result of a single-drive path lookup. When
// the path crosses a mount node, Remaining holds the path components
// that follow it; the caller (Service.Resolve) uses this to follow
// the mount into the source drive.
type resolveOutcome struct {
	Node      *node.Node
	Remaining string
}

// resolve walks from root to the node at the given absolute path.
// Supports "." and ".." per POSIX (no ascending above the drive
// root). Stops at the first mount node: the mount itself is
// returned and Remaining contains the rest of the path for the
// caller to follow.
func (r *resolver) resolve(ctx context.Context, rootID uuid.UUID, p string) (resolveOutcome, error) {
	cleaned := cleanPath(p)
	if cleaned == "" || cleaned == "/" {
		n, err := r.node.GetByID(ctx, rootID)
		if err != nil {
			return resolveOutcome{}, err
		}
		return resolveOutcome{Node: n}, nil
	}
	current, err := r.node.GetByID(ctx, rootID)
	if err != nil || current == nil {
		return resolveOutcome{}, ErrNotFound
	}
	ancestors := []*node.Node{current}
	parts := splitPath(cleaned)
	for i, part := range parts {
		switch part {
		case "", ".":
			continue
		case "..":
			if len(ancestors) <= 1 {
				return resolveOutcome{}, ErrInvalidPath
			}
			ancestors = ancestors[:len(ancestors)-1]
			current = ancestors[len(ancestors)-1]
			continue
		}
		de, err := current.Lookup(part)
		if err != nil {
			return resolveOutcome{}, err
		}
		if de == nil {
			return resolveOutcome{}, ErrNotFound
		}
		child, err := r.node.GetByID(ctx, de.InodeID)
		if err != nil || child == nil {
			return resolveOutcome{}, ErrNotFound
		}
		if child.IsMount() {
			rest := joinParts(parts[i+1:])
			return resolveOutcome{Node: child, Remaining: rest}, nil
		}
		ancestors = append(ancestors, child)
		current = child
	}
	return resolveOutcome{Node: current}, nil
}

// resolve walks from root to the node at the given absolute path,
// transparently following mount nodes along the way. Returns the
// drive the final node lives in (which may differ from driveID if
// mounts were crossed) and the node itself. Cycles in the mount
// graph are detected via a visited set; maxMountHops is a safety net
// against pathological graphs.
func (s *Service) resolve(ctx context.Context, driveID, path string) (driveIDOut string, n *node.Node, err error) {
	visited := map[string]struct{}{driveID: {}}
	currentDrive := driveID
	currentPath := cleanPath(path)
	for hop := 0; hop < maxMountHops; hop++ {
		rootID, err := s.rootNodeID(ctx, currentDrive)
		if err != nil {
			return "", nil, err
		}
		out, err := s.path.resolve(ctx, rootID, currentPath)
		if err != nil {
			return "", nil, err
		}
		if out.Remaining == "" {
			return currentDrive, out.Node, nil
		}
		// The result is a mount node. Follow it into the source drive
		// and continue with whatever path was to the right of it.
		srcDriveID, err := out.Node.ReadMount()
		if err != nil {
			return "", nil, err
		}
		if _, seen := visited[srcDriveID]; seen {
			return "", nil, ErrMountCycle
		}
		visited[srcDriveID] = struct{}{}
		currentDrive = srcDriveID
		currentPath = "/" + out.Remaining
	}
	return "", nil, ErrPathTooDeep
}

// resolveParent returns the parent node and the last path component.
// For paths that cross a mount, the returned parent is the mount
// node itself; cross-drive continuation is the caller's
// responsibility (e.g. mv constructs both endpoints explicitly).
func (r *resolver) resolveParent(ctx context.Context, rootID uuid.UUID, p string) (parent *node.Node, name string, err error) {
	cleaned := cleanPath(p)
	idx := strings.LastIndex(cleaned, "/")
	if idx < 0 {
		return nil, "", ErrInvalidPath
	}
	parentPath := cleaned[:idx]
	if parentPath == "" {
		parentPath = "/"
	}
	name = cleaned[idx+1:]
	if name == "" {
		return nil, "", ErrInvalidPath
	}
	out, err := r.resolve(ctx, rootID, parentPath)
	if err != nil {
		return nil, "", err
	}
	return out.Node, name, nil
}

// splitPath splits a cleaned, absolute path into non-empty components.
func splitPath(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

// joinParts concatenates path components with "/". Used to
// reconstruct the trailing path when a resolver stops at a mount.
func joinParts(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += "/"
		}
		out += p
	}
	return out
}

// cleanPath normalizes an absolute path. Uses path.Clean for collapse
// of "."/".." and repeated slashes, then ensures the result is
// absolute.
func cleanPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "/"
	}
	cleaned := path.Clean("/" + p)
	if cleaned == "" || cleaned == "." {
		return "/"
	}
	return cleaned
}
