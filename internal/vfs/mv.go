package vfs

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/mandacode-labs/mdrive/internal/permission"
)

// Mv moves sources to dest (like `mv src1 src2 ... dest/`).
// Same-drive only for now.
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

	// Resolve destination parent.
	dstParent, dstName, err := s.path.resolveParent(ctx, rootID, dstPath)
	if err != nil {
		// If dst doesn't exist yet and there's only one src, create as rename.
		if len(srcPaths) == 1 {
			return s.mvRename(ctx, rootID, srcPaths[0], dstPath)
		}
		return fmt.Errorf("mv: dest: %w", err)
	}

	// Move each source into the destination directory.
	for _, srcPath := range srcPaths {
		src, err := s.path.resolve(ctx, rootID, srcPath)
		if err != nil {
			return fmt.Errorf("mv: %s: %w", srcPath, err)
		}
		srcParent, srcName, _ := s.path.resolveParent(ctx, rootID, srcPath)
		if srcParent != nil {
			_ = s.Node.Unlink(ctx, srcParent, srcName)
		}
		if err := s.Node.Link(ctx, dstParent, dstName, src); err != nil {
			if srcParent != nil {
				_ = s.Node.Link(ctx, srcParent, srcName, src)
			}
			return fmt.Errorf("mv: link: %w", err)
		}
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
	srcParent, srcName, _ := s.path.resolveParent(ctx, rootID, srcPath)
	if srcParent != nil {
		_ = s.Node.Unlink(ctx, srcParent, srcName)
	}
	if err := s.Node.Link(ctx, dstParent, dstName, src); err != nil {
		if srcParent != nil {
			_ = s.Node.Link(ctx, srcParent, srcName, src)
		}
		return fmt.Errorf("mv: link: %w", err)
	}
	return nil
}
