package vfs

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/mandacode-labs/mdrive/internal/core/node"
)

// Resolver walks the node tree from a drive root to resolve Unix paths.
type Resolver struct {
	nodeSvc *node.Service
}

func newResolver(nodeSvc *node.Service) *Resolver {
	return &Resolver{nodeSvc: nodeSvc}
}

// resolve walks from root to the node at the given absolute path.
func (r *Resolver) resolve(ctx context.Context, rootID uuid.UUID, path string) (*node.Node, error) {
	path = cleanPath(path)
	if path == "" || path == "/" {
		return r.nodeSvc.GetByID(ctx, rootID)
	}
	current, err := r.nodeSvc.GetByID(ctx, rootID)
	if err != nil || current == nil {
		return nil, ErrNotFound
	}
	for _, part := range splitPath(path) {
		de, err := current.Lookup(part)
		if err != nil {
			return nil, err
		}
		if de == nil {
			return nil, ErrNotFound
		}
		current, err = r.nodeSvc.GetByID(ctx, de.InodeID)
		if err != nil || current == nil {
			return nil, ErrNotFound
		}
	}
	return current, nil
}

// resolveParent returns the parent node and the last path component.
func (r *Resolver) resolveParent(ctx context.Context, rootID uuid.UUID, path string) (parent *node.Node, name string, err error) {
	path = cleanPath(path)
	idx := strings.LastIndex(path, "/")
	if idx < 0 {
		return nil, "", ErrInvalidPath
	}
	parentPath := path[:idx]
	if parentPath == "" {
		parentPath = "/"
	}
	name = path[idx+1:]
	if name == "" {
		return nil, "", ErrInvalidPath
	}
	parent, err = r.resolve(ctx, rootID, parentPath)
	return
}

// splitPath splits a cleaned, absolute path into non-empty components.
func splitPath(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}

// cleanPath normalizes an absolute path.
func cleanPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/"
	}
	// Simple normalization: remove double slashes, trailing slash (keep root).
	for strings.Contains(path, "//") {
		path = strings.ReplaceAll(path, "//", "/")
	}
	if len(path) > 1 && strings.HasSuffix(path, "/") {
		path = path[:len(path)-1]
	}
	return path
}
