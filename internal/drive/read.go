package drive

import (
	"context"

	coredrive "github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/permission"
)

// Get returns a drive by private ID. Requires view permission.
func (s *Service) Get(ctx context.Context, actorID, id string) (*coredrive.Drive, error) {
	if err := s.checkAccess(ctx, actorID, permission.PermissionView, id); err != nil {
		return nil, err
	}
	return s.Drive.GetByID(ctx, id)
}

// GetByPublicID returns a drive by public ID. No permission check
// (the public ID is the share token).
func (s *Service) GetByPublicID(ctx context.Context, pubID string) (*coredrive.Drive, error) {
	return s.Drive.GetByPublicID(ctx, pubID)
}

// GetStorage returns the storage config for a drive. Requires view.
func (s *Service) GetStorage(ctx context.Context, actorID, driveID string) (*coredrive.Storage, error) {
	if err := s.checkAccess(ctx, actorID, permission.PermissionView, driveID); err != nil {
		return nil, err
	}
	return s.Drive.GetStorage(ctx, driveID)
}
