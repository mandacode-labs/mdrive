package vfs

import (
	"context"
	"fmt"

	"github.com/mandacode-labs/mdrive/internal/permission"
)

// Mv moves src to dst (like `mv /src /dst`). Same-drive only for now.
func (s *Service) Mv(ctx context.Context, userID, srcDriveID, srcPath, dstDriveID, dstPath string) error {
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
		_ = s.node.Unlink(ctx, srcParent, srcName)
	}
	if err := s.node.Link(ctx, dstParent, dstName, src); err != nil {
		if srcParent != nil {
			_ = s.node.Link(ctx, srcParent, srcName, src)
		}
		return fmt.Errorf("mv: link: %w", err)
	}
	return nil
}
