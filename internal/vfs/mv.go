package vfs

import (
	"context"
	"fmt"

	"github.com/mandacode-labs/mdrive/internal/permission"
)

// Mv moves src to dst (like `mv /src /dst`).
func (s *Service) Mv(ctx context.Context, userID, srcDriveID, srcPath, dstDriveID, dstPath string) error {
	if srcDriveID != dstDriveID {
		return ErrCrossDrive
	}
	if err := s.checkAccess(ctx, userID, permission.PermissionEdit, srcDriveID); err != nil {
		return err
	}
	d := s.mustGetDrive(ctx, srcDriveID)
	if d == nil {
		return ErrNotFound
	}
	src, err := s.path.resolve(ctx, *d.RootNodeID(), srcPath)
	if err != nil {
		return fmt.Errorf("mv: src: %w", err)
	}
	dstParent, dstName, err := s.path.resolveParent(ctx, *d.RootNodeID(), dstPath)
	if err != nil {
		return fmt.Errorf("mv: dst: %w", err)
	}
	srcParent, srcName, _ := s.path.resolveParent(ctx, *d.RootNodeID(), srcPath)

	if srcParent != nil {
		_ = s.nodeSvc.Unlink(ctx, srcParent, srcName)
	}
	if err := s.nodeSvc.Link(ctx, dstParent, dstName, src); err != nil {
		if srcParent != nil {
			_ = s.nodeSvc.Link(ctx, srcParent, srcName, src)
		}
		return fmt.Errorf("mv: link: %w", err)
	}
	return nil
}
