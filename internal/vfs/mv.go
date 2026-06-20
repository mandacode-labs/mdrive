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
		for _, srcPath := range srcPaths {
			if err := tx.mvOne(ctx, rootID, srcPath, dstParent, dstName); err != nil {
				return err
			}
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
		if err := s.Node.Unlink(ctx, srcParent, srcName); err != nil {
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
		if err := s.Node.Unlink(ctx, srcParent, srcName); err != nil {
			return fmt.Errorf("mv: unlink: %w", err)
		}
	}
	if err := s.Node.Link(ctx, dstParent, dstName, src); err != nil {
		return fmt.Errorf("mv: link: %w", err)
	}
	return nil
}

// overwriteTarget removes the existing entry at dstParent/dstName if it exists.
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
	if err := s.Node.Unlink(ctx, parent, name); err != nil {
		return fmt.Errorf("overwrite: unlink existing: %w", err)
	}
	if err := s.Node.Delete(ctx, existing.ID()); err != nil {
		return fmt.Errorf("overwrite: delete existing: %w", err)
	}
	return nil
}
