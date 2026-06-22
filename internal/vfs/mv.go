package vfs

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/permission"
)

// Mv moves sources to dest (like `mv src1 src2 ... dest/`).
// Same-drive only. Executes within a node transaction so partial
// moves are automatically rolled back. If the destination exists as a file,
// it is overwritten (POSIX behavior).
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
		dstParent, dstName, err := tx.path.resolveParent(ctx, rootID, dstPath)
		if err != nil {
			if len(srcPaths) == 1 && errors.Is(err, ErrNotFound) {
				return tx.mvRename(ctx, rootID, srcPaths[0], dstPath)
			}
			return fmt.Errorf("mv: dest: %w", err)
		}
		if !dstParent.IsDir() {
			return fmt.Errorf("mv: dest: %w", ErrNotDirectory)
		}
		// Resolve every source upfront and validate. We collect the
		// (name -> node) pairs so a single BulkLink can write all entries
		// at once, and a single BulkUnlink can detach them from their
		// old parents in one round-trip per parent.
		type src struct {
			node       *node.Node
			srcParent  *node.Node
			srcName    string
		}
		sources := make([]src, 0, len(srcPaths))
		for _, srcPath := range srcPaths {
			n, err := tx.path.resolve(ctx, rootID, srcPath)
			if err != nil {
				return fmt.Errorf("mv: %s: %w", srcPath, err)
			}
			srcParent, srcName, err := tx.path.resolveParent(ctx, rootID, srcPath)
			if err != nil {
				return fmt.Errorf("mv: %s: resolve parent: %w", srcPath, err)
			}
			sources = append(sources, src{node: n, srcParent: srcParent, srcName: srcName})
		}
		// Overwrite target, if any. Done once for the whole batch.
		if err := tx.overwriteTarget(ctx, dstParent, dstName); err != nil {
			return fmt.Errorf("mv: overwrite target: %w", err)
		}
		// Group unlinks by source parent so we can use BulkUnlink.
		byParent := make(map[*node.Node][]string)
		linkEntries := make(map[string]*node.Node, len(sources))
		for _, s := range sources {
			if s.srcParent != nil && s.srcName != "" {
				byParent[s.srcParent] = append(byParent[s.srcParent], s.srcName)
			}
			linkEntries[dstName+"_"+s.srcName] = s.node
		}
		// If the destination name collides with one of the source names
		// (mv a b a/) is rejected upfront to avoid double-adding the
		// same name; we keep distinct keys in linkEntries so this works
		// for distinct sources, but dstName collisions need a different
		// strategy. For now require unique dstName per batch.
		seen := make(map[string]struct{}, len(sources))
		uniqueLinks := make(map[string]*node.Node, len(sources))
		for _, s := range sources {
			if _, dup := seen[dstName]; dup {
				return fmt.Errorf("mv: duplicate destination name %q in batch", dstName)
			}
			seen[dstName] = struct{}{}
			uniqueLinks[dstName] = s.node
			_ = linkEntries
		}
		// Unlink from old parents (bulk per parent).
		for parent, names := range byParent {
			if _, err := tx.Node.BulkUnlink(ctx, parent, names); err != nil {
				return fmt.Errorf("mv: bulk unlink: %w", err)
			}
		}
		// Link into the destination (single bulk write).
		if err := tx.Node.BulkLink(ctx, dstParent, uniqueLinks); err != nil {
			return fmt.Errorf("mv: bulk link: %w", err)
		}
		return nil
	})
}

// mvOne moves a single source into dstParent with the given name.
// If a file already exists at the destination, it is overwritten.
func (s *Service) mvOne(ctx context.Context, rootID uuid.UUID, srcPath string, dstParent *node.Node, dstName string) error {
	src, err := s.path.resolve(ctx, rootID, srcPath)
	if err != nil {
		return fmt.Errorf("mv: %s: %w", srcPath, err)
	}
	srcParent, srcName, err := s.path.resolveParent(ctx, rootID, srcPath)
	if err != nil {
		return fmt.Errorf("mv: resolve src parent: %w", err)
	}
	if err := s.overwriteTarget(ctx, dstParent, dstName); err != nil {
		return fmt.Errorf("mv: overwrite target: %w", err)
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

// mvRename handles the single-source case where dstPath is the new name.
func (s *Service) mvRename(ctx context.Context, rootID uuid.UUID, srcPath, dstPath string) error {
	src, err := s.path.resolve(ctx, rootID, srcPath)
	if err != nil {
		return fmt.Errorf("mv: src: %w", err)
	}
	dstParent, dstName, err := s.path.resolveParent(ctx, rootID, dstPath)
	if err != nil {
		return fmt.Errorf("mv: dst: %w", err)
	}
	if err := s.overwriteTarget(ctx, dstParent, dstName); err != nil {
		return fmt.Errorf("mv: overwrite target: %w", err)
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

// overwriteTarget removes the existing entry at dstParent/dstName if it exists.
// Delegates nlink management to node.Service.UnlinkOrReplace so that
// hardlinks are decremented correctly and only deleted when nlink==0.
func (s *Service) overwriteTarget(ctx context.Context, parent *node.Node, name string) error {
	entry, err := parent.Lookup(name)
	if err != nil {
		return nil // no existing entry, nothing to overwrite
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
