package vfs

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/permission"
)

// Mv moves sources to dest (like `mv src1 src2 ... dest/`).
// Same-drive only. Executes within a node transaction so partial moves
// are automatically rolled back. If the destination entry exists as a
// file, it is overwritten (POSIX behavior).
//
// Semantics:
//   - Single source + non-existent destination path: rename.
//   - Single source + existing-directory destination: move into it.
//   - Multiple sources: destination must be an existing directory;
//     each source's basename is used.
func (s *Service) Mv(ctx context.Context, userID, srcDriveID string, srcPaths []string, dstDriveID, dstPath string) error {
	if srcDriveID != dstDriveID {
		return ErrCrossDrive
	}
	if err := s.checkAccess(ctx, userID, permission.PermissionEdit, srcDriveID); err != nil {
		return err
	}
	rootID, err := s.rootNodeID(ctx, srcDriveID)
	if err != nil {
		return err
	}

	return s.WithNodeTx(ctx, func(tx *Service) error {
		if len(srcPaths) == 1 {
			return tx.mvSingle(ctx, rootID, srcPaths[0], dstPath)
		}
		return tx.mvBatch(ctx, rootID, srcPaths, dstPath)
	})
}

// mvSingle handles the single-source case. The destination is either
// a rename target (last path component does not exist yet) or an
// existing directory.
func (s *Service) mvSingle(ctx context.Context, rootID uuid.UUID, srcPath, dstPath string) error {
	dstParent, dstName, err := s.path.resolveParent(ctx, rootID, dstPath)
	if err != nil {
		return fmt.Errorf("mv: dest: %w", err)
	}
	if !dstParent.IsDir() {
		return fmt.Errorf("mv: dest: %w", ErrNotDirectory)
	}
	if err := s.overwriteTarget(ctx, dstParent, dstName); err != nil {
		return fmt.Errorf("mv: overwrite target: %w", err)
	}
	return s.mvRename(ctx, rootID, srcPath, dstParent, dstName)
}

// mvBatch handles multi-source moves. The destination must resolve to
// an existing directory; sources are moved in keeping their basenames.
func (s *Service) mvBatch(ctx context.Context, rootID uuid.UUID, srcPaths []string, dstPath string) error {
	dstDir, err := s.path.resolve(ctx, rootID, dstPath)
	if err != nil {
		return fmt.Errorf("mv: dest: %w", err)
	}
	if !dstDir.IsDir() {
		return fmt.Errorf("mv: dest: %w", ErrNotDirectory)
	}

	type srcInfo struct {
		node      *node.Node
		baseName  string
		srcParent *node.Node
		srcName   string
	}
	sources := make([]srcInfo, 0, len(srcPaths))
	seen := make(map[string]struct{}, len(srcPaths))
	for _, srcPath := range srcPaths {
		n, err := s.path.resolve(ctx, rootID, srcPath)
		if err != nil {
			return fmt.Errorf("mv: %s: %w", srcPath, err)
		}
		sp, sn, err := s.path.resolveParent(ctx, rootID, srcPath)
		if err != nil {
			return fmt.Errorf("mv: %s: resolve parent: %w", srcPath, err)
		}
		base := sn
		if base == "" {
			base = srcPath
		}
		if _, dup := seen[base]; dup {
			return fmt.Errorf("mv: duplicate source basename %q in batch", base)
		}
		seen[base] = struct{}{}
		sources = append(sources, srcInfo{node: n, baseName: base, srcParent: sp, srcName: sn})
	}

	// Reject any source-destination collision up front: a source cannot
	// be moved into a directory that is also one of its ancestors
	// (would create a cycle).
	for _, si := range sources {
		if si.node.ID() == dstDir.ID() {
			return fmt.Errorf("mv: cannot move directory into itself")
		}
	}

	// Handle existing destination entries: overwrite files, reject dirs.
	links := make(map[string]*node.Node, len(sources))
	for _, si := range sources {
		entry, err := dstDir.Lookup(si.baseName)
		if err != nil {
			return fmt.Errorf("mv: dst lookup: %w", err)
		}
		if entry != nil {
			existing, err := s.Node.GetByID(ctx, entry.InodeID)
			if err != nil {
				return fmt.Errorf("mv: dst existing: %w", err)
			}
			if existing.IsDir() {
				return fmt.Errorf("mv: cannot overwrite directory %q with a non-directory", si.baseName)
			}
			if _, err := s.Node.UnlinkOrReplace(ctx, dstDir, si.baseName); err != nil {
				return fmt.Errorf("mv: dst unlink: %w", err)
			}
		}
		links[si.baseName] = si.node
	}

	// Detach sources from their old parents in bulk.
	byParent := make(map[*node.Node][]string)
	for _, si := range sources {
		if si.srcParent != nil && si.srcName != "" {
			byParent[si.srcParent] = append(byParent[si.srcParent], si.srcName)
		}
	}
	for parent, names := range byParent {
		if _, err := s.Node.BulkUnlink(ctx, parent, names); err != nil {
			return fmt.Errorf("mv: bulk unlink: %w", err)
		}
	}

	// Link into the destination with one bulk write.
	if err := s.Node.BulkLink(ctx, dstDir, links); err != nil {
		return fmt.Errorf("mv: bulk link: %w", err)
	}
	return nil
}

// mvRename moves a single source to dstParent/dstName. The source is
// unlinked from its old parent and re-linked in the new parent within
// the same transaction.
func (s *Service) mvRename(ctx context.Context, rootID uuid.UUID, srcPath string, dstParent *node.Node, dstName string) error {
	src, err := s.path.resolve(ctx, rootID, srcPath)
	if err != nil {
		return fmt.Errorf("mv: src %s: %w", srcPath, err)
	}
	srcParent, srcName, err := s.path.resolveParent(ctx, rootID, srcPath)
	if err != nil {
		return fmt.Errorf("mv: resolve src parent: %w", err)
	}
	if srcParent != nil && srcName != "" {
		if _, err := s.Node.Unlink(ctx, srcParent, srcName); err != nil {
			return fmt.Errorf("mv: unlink: %w", err)
		}
	}
	if err := s.Node.Link(ctx, dstParent, dstName, src); err != nil {
		return fmt.Errorf("mv: link: %w", err)
	}
	return nil
}

// overwriteTarget removes the existing entry at parent/name if it
// exists. Directories are rejected (POSIX: cannot overwrite a directory
// with a non-directory). Delegates nlink management to
// node.Service.UnlinkOrReplace so hardlinks are decremented and only
// deleted when nlink==0.
func (s *Service) overwriteTarget(ctx context.Context, parent *node.Node, name string) error {
	entry, err := parent.Lookup(name)
	if err != nil {
		return nil
	}
	if entry == nil {
		return nil
	}
	existing, err := s.Node.GetByID(ctx, entry.InodeID)
	if err != nil {
		return fmt.Errorf("overwrite: get existing: %w", err)
	}
	if existing.IsDir() {
		return fmt.Errorf("cannot overwrite directory %q with a non-directory", name)
	}
	_, err = s.Node.UnlinkOrReplace(ctx, parent, name)
	if err != nil {
		return fmt.Errorf("overwrite: unlink: %w", err)
	}
	return nil
}
