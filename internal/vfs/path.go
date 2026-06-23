package vfs

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/google/uuid"
	"github.com/mandacode-labs/mdrive/internal/core/node"
)

// resolver walks the node tree from a drive root to resolve Unix
// paths. Each resolver instance has its own cache that memoizes
// GetByID loads within the instance, so multiple resolves of the
// same UUID return the same in-memory *Node pointer. This pointer
// identity is required for the optimistic-concurrency check in
// node.Repository.Save (staleRev) to behave consistently within
// one operation.
//
// Resolvers are NOT safe to share across operations: the cache
// grows without bounds and stale entries may bleed into a
// later call. Each vfs method that resolves more than once
// must construct its own resolver.
type resolver struct {
	node  NodeClient
	cache map[uuid.UUID]*node.Node
}

func newResolver(n NodeClient) *resolver {
	return &resolver{node: n, cache: make(map[uuid.UUID]*node.Node)}
}

// loadByID returns the cached *Node for id, or fetches it via the
// NodeClient and caches the result. Two loads of the same UUID
// within one resolver always return the same pointer.
func (r *resolver) loadByID(ctx context.Context, id uuid.UUID) (*node.Node, error) {
	if n, ok := r.cache[id]; ok {
		return n, nil
	}
	n, err := r.node.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	r.cache[id] = n
	return n, nil
}

// resolveOutcome is the result of a single-drive path lookup. When
// the path crosses a mount node, Remaining holds the path components
// that follow it; the caller (Service.Resolve) uses this to follow
// the mount into the source drive.
type resolveOutcome struct {
	Node      *node.Node
	Remaining string
}

// maxSymlinkDepth mirrors Linux's MAXSYMLINKS (40). Path resolution
// stops with ErrSymlinkCycle if the symlink-follow budget is
// exhausted, matching POSIX ELOOP.
const maxSymlinkDepth = 40

// resolve walks from root to the node at the given absolute path,
// following symlinks per POSIX when follow is true (Linux stat(2)
// semantics). When follow is false, the resolution stops at the
// first symlink and returns it (Linux lstat(2) semantics).
//
// Supports "." and "..". Stops at the first mount node: the mount
// itself is returned and Remaining contains the rest of the path
// for the caller to follow into the source drive.
//
// Symlink loops are detected by tracking the inodes of symlinks
// traversed during a single follow chain (POSIX ELOOP).
//
// Relative symlinks resolve relative to their parent directory;
// absolute targets restart resolution from the drive root.
func (r *resolver) resolve(ctx context.Context, rootID uuid.UUID, p string, follow bool) (resolveOutcome, error) {
	return r.resolveInner(ctx, rootID, p, follow, make(map[uuid.UUID]struct{}))
}

// resolveInner does the actual walk. It is split out from resolve
// so the symlink-follow branch can recurse (cheap Go call, not
// a Go-statement-cost loop iteration) when a target needs to be
// spliced into the remaining path.
func (r *resolver) resolveInner(ctx context.Context, rootID uuid.UUID, p string, follow bool, visited map[uuid.UUID]struct{}) (resolveOutcome, error) {
	cleaned := cleanPath(p)
	if cleaned == "" || cleaned == "/" {
		n, err := r.loadByID(ctx, rootID)
		if err != nil {
			return resolveOutcome{}, err
		}
		return resolveOutcome{Node: n}, nil
	}
	root, err := r.loadByID(ctx, rootID)
	if err != nil || root == nil {
		return resolveOutcome{}, ErrNotFound
	}
	cur := root
	parents := []*node.Node{root}
	parts := splitPath(cleaned)
	for i, part := range parts {
		switch part {
		case "", ".":
			continue
		case "..":
			if len(parents) <= 1 {
				return resolveOutcome{}, ErrInvalidPath
			}
			parents = parents[:len(parents)-1]
			cur = parents[len(parents)-1]
			continue
		}

		de, err := cur.Lookup(part)
		if err != nil {
			return resolveOutcome{}, err
		}
		if de == nil {
			return resolveOutcome{}, ErrNotFound
		}
		child, err := r.loadByID(ctx, de.InodeID)
		if err != nil || child == nil {
			return resolveOutcome{}, ErrNotFound
		}
		if child.IsMount() {
			rest := joinParts(parts[i+1:])
			return resolveOutcome{Node: child, Remaining: rest}, nil
		}

		// Symlink follow: if the child we just resolved is a
		// symlink and follow is on, splice the target into the
		// remaining path and recurse from the drive root.
		if follow && child.IsSymlink() {
			if len(visited) >= maxSymlinkDepth {
				return resolveOutcome{}, node.ErrSymlinkCycle
			}
			if _, seen := visited[child.ID()]; seen {
				return resolveOutcome{}, node.ErrSymlinkCycle
			}
			visited[child.ID()] = struct{}{}

			target, err := child.Readlink()
			if err != nil {
				return resolveOutcome{}, err
			}
			suffix := joinParts(parts[i+1:])
			var joined string
			if strings.HasPrefix(target, "/") {
				if suffix == "" {
					joined = target
				} else {
					joined = target + "/" + suffix
				}
			} else {
				parentParts := parts[:i]
				parentPath := joinParts(parentParts)
				if parentPath == "" {
					parentPath = "/"
				}
				if suffix == "" {
					joined = path.Clean(parentPath + "/" + target)
				} else {
					joined = path.Clean(parentPath + "/" + target + "/" + suffix)
				}
			}
			return r.resolveInner(ctx, rootID, joined, follow, visited)
		}

		parents = append(parents, child)
		cur = child
	}
	return resolveOutcome{Node: cur}, nil
}

// resolveCross walks from root to the node at the given absolute path,
// transparently following mount nodes along the way. Returns the
// drive the final node lives in (which may differ from driveID if
// mounts were crossed) and the node itself. Cycles in the mount
// graph are detected via a visited set; maxMountHops is a safety net
// against pathological graphs.
//
// Each hop uses a fresh resolver so the cache for one mount
// traversal does not leak across hops.
func (s *Service) resolveCross(ctx context.Context, driveID, path string, follow bool) (driveIDOut string, n *node.Node, err error) {
	visited := map[string]struct{}{driveID: {}}
	currentDrive := driveID
	currentPath := cleanPath(path)
	for hop := 0; hop < maxMountHops; hop++ {
		r := s.newResolver()
		rootID, err := s.rootNodeID(ctx, currentDrive)
		if err != nil {
			return "", nil, err
		}
		out, err := r.resolve(ctx, rootID, currentPath, follow)
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

// resolveParent returns the parent node and the last path component
// of p, following symlinks in the parent portion (so the returned
// parent is the actual directory that would contain the new entry,
// not a symlink in the middle of the parent path). Symlinks at
// the leaf name itself are NOT followed: the leaf is what the caller
// intends to operate on. Stops at the first mount node along the
// parent path; cross-drive continuation is the caller's
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
	out, err := r.resolve(ctx, rootID, parentPath, true)
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

// requireEditPath resolves the path's parent directory and verifies
// it is a directory. Returns ErrNotDirectory if the parent is not
// a directory.
//
// Permission is the caller's responsibility: vfs does not check
// edit permission.
func (s *Service) requireEditPath(ctx context.Context, driveID, path string) (parent *node.Node, name string, err error) {
	rootID, err := s.rootNodeID(ctx, driveID)
	if err != nil {
		return nil, "", err
	}
	parent, name, err = s.newResolver().resolveParent(ctx, rootID, path)
	if err != nil {
		return nil, "", err
	}
	if !parent.IsDir() {
		return nil, "", ErrNotDirectory
	}
	return parent, name, nil
}

// createAndLink links child at parent/name. On link failure, the
// child is deleted to prevent leaking unparented inodes; the
// original link error is returned with cleanup context.
func (s *Service) createAndLink(ctx context.Context, child, parent *node.Node, name string) error {
	if err := s.Node.Link(ctx, parent, name, child); err != nil {
		if derr := s.Node.Delete(ctx, child.ID()); derr != nil {
			return fmt.Errorf("link: %w (cleanup: %v)", err, derr)
		}
		return fmt.Errorf("link: %w", err)
	}
	return nil
}
