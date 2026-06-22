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

// resolve walks from root to the node at the given absolute path.
// It supports "." and ".." components per POSIX semantics: traversal
// cannot ascend above the drive root.
func (r *resolver) resolve(ctx context.Context, rootID uuid.UUID, p string) (*node.Node, error) {
	cleaned := cleanPath(p)
	if cleaned == "" || cleaned == "/" {
		return r.node.GetByID(ctx, rootID)
	}
	current, err := r.node.GetByID(ctx, rootID)
	if err != nil || current == nil {
		return nil, ErrNotFound
	}
	// Track an ancestor chain so ".." doesn't need a second round-trip
	// per step. We always keep the chain rooted at the drive root.
	ancestors := []*node.Node{current}
	for _, part := range splitPath(cleaned) {
		switch part {
		case "", ".":
			continue
		case "..":
			if len(ancestors) <= 1 {
				// At the drive root; refuse to ascend further.
				return nil, ErrInvalidPath
			}
			ancestors = ancestors[:len(ancestors)-1]
			current = ancestors[len(ancestors)-1]
			continue
		}
		de, err := current.Lookup(part)
		if err != nil {
			return nil, err
		}
		if de == nil {
			return nil, ErrNotFound
		}
		current, err = r.node.GetByID(ctx, de.InodeID)
		if err != nil || current == nil {
			return nil, ErrNotFound
		}
		ancestors = append(ancestors, current)
	}
	return current, nil
}

// resolveParent returns the parent node and the last path component.
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
	parent, err = r.resolve(ctx, rootID, parentPath)
	return
}

// splitPath splits a cleaned, absolute path into non-empty components.
func splitPath(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

// cleanPath normalizes an absolute path. Uses path.Clean for collapse
// of "."/".." and repeated slashes, then ensures the result is absolute.
func cleanPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "/"
	}
	// path.Clean returns "." for empty input; force "/" for our callers.
	cleaned := path.Clean("/" + p)
	if cleaned == "" || cleaned == "." {
		return "/"
	}
	return cleaned
}
