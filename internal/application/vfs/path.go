package vfs

import (
	"context"
	"path"
	"strings"

	"github.com/mandacode-labs/mdrive/internal/core/inode"
	"github.com/mandacode-labs/mdrive/internal/errors"
)

// GetRootDirectory returns the root directory for a system.
// The root directory is identified by the FlagRoot flag.
func (s *service) GetRootDirectory(ctx context.Context, systemID string) (*inode.Inode, error) {
	inodes, err := s.inodeOps.Find(ctx, inode.Filter{
		SystemID: &systemID,
	})
	if err != nil {
		return nil, err
	}

	for _, in := range inodes {
		if in.Flags()&inode.FlagRoot != 0 && in.IsDir() {
			return in, nil
		}
	}

	return nil, errors.NotFound("root directory not found")
}

// ResolvePath resolves a Unix-style path to an inode.
// Path must be absolute (start with /).
// Example: /home/user/file.txt
func (s *service) ResolvePath(ctx context.Context, systemID string, pathStr string) (*inode.Inode, error) {
	if pathStr == "" || pathStr[0] != '/' {
		return nil, errors.BadRequest("path must be absolute (start with /)")
	}

	// Get root directory
	root, err := s.GetRootDirectory(ctx, systemID)
	if err != nil {
		return nil, err
	}

	// Special case: root path
	if pathStr == "/" {
		return root, nil
	}

	// Split path into components
	// path.Clean handles . and .. and trailing slashes
	cleanPath := path.Clean(pathStr)
	components := strings.Split(cleanPath, "/")
	// First element is empty (before leading /)
	components = components[1:]

	// Traverse from root
	current := root
	for _, component := range components {
		if component == "" {
			continue
		}

		// Read directory entries
		entries, err := current.ReadDir()
		if err != nil {
			return nil, err
		}

		// Find the entry with matching name
		found := false
		for _, entry := range entries {
			if entry.Name == component {
				// Get the inode for this entry
				child, err := s.inodeOps.GetByID(ctx, entry.InodeID)
				if err != nil {
					return nil, err
				}

				// Follow symlinks
				if child.IsSymlink() {
					// Resolve symlink target
					target, err := s.resolveSymlink(ctx, child)
					if err != nil {
						return nil, err
					}
					child = target
				}

				current = child
				found = true
				break
			}
		}

		if !found {
			return nil, errors.NotFound("path component not found: " + component)
		}
	}

	return current, nil
}

// resolveSymlink resolves a symlink to its target inode.
func (s *service) resolveSymlink(ctx context.Context, sym *inode.Inode) (*inode.Inode, error) {
	targetPath, err := sym.SymlinkTarget()
	if err != nil {
		return nil, err
	}

	// Resolve the target path
	return s.ResolvePath(ctx, sym.SystemID(), targetPath)
}
