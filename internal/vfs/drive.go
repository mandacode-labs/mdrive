package vfs

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/permission"
)

// Drive operations — delegates to the drive domain service and adds permission checks.

// CreateDrive creates a drive with storage config and root node.
// Grants the owner relation in OpenFGA.
func (s *Service) CreateDrive(ctx context.Context, actorID string, name, description string, cfg drive.StorageConfig) (*drive.Drive, uuid.UUID, error) {
	d, rootID, err := s.driveSvc.Create(ctx, name, &description, actorID, cfg)
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("vfs: %w", err)
	}
	// Grant owner in OpenFGA (non-fatal if this fails).
	_ = s.perm.Grant(ctx, actorID, permission.RelationOwner, permission.ObjectTypeDrive, d.ID())
	return d, rootID, nil
}

// GetDrive returns a drive by private ID.
func (s *Service) GetDrive(ctx context.Context, id string) (*drive.Drive, error) {
	d, err := s.driveSvc.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("vfs: %w", err)
	}
	return d, nil
}

// GetDriveByPublicID returns a drive by public ID.
func (s *Service) GetDriveByPublicID(ctx context.Context, publicID string) (*drive.Drive, error) {
	return s.driveSvc.GetByPublicID(ctx, publicID)
}

// GetDriveStorage returns the storage config for a drive.
func (s *Service) GetDriveStorage(ctx context.Context, driveID string) (*drive.Storage, error) {
	return s.driveSvc.GetStorage(ctx, driveID)
}

// UpdateDrive updates mutable drive fields.
func (s *Service) UpdateDrive(ctx context.Context, id string, name, description *string) (*drive.Drive, error) {
	return s.driveSvc.Update(ctx, id, name, description)
}

// DeleteDrive removes a drive. Caller must first walk nodes and clean up S3.
func (s *Service) DeleteDrive(ctx context.Context, id string) error {
	return s.driveSvc.Delete(ctx, id)
}

// ListUserDrives returns all drives owned by actorID.
func (s *Service) ListUserDrives(ctx context.Context, actorID string) ([]*drive.Drive, error) {
	return s.driveSvc.ListByOwner(ctx, actorID)
}

// GrantDriveAccess grants a role to a user on a drive.
func (s *Service) GrantDriveAccess(ctx context.Context, _, userID, driveID, relation string) error {
	return s.perm.Grant(ctx, userID, relation, permission.ObjectTypeDrive, driveID)
}

// RevokeDriveAccess revokes all relations for a user on a drive.
func (s *Service) RevokeDriveAccess(ctx context.Context, _, userID, driveID string) error {
	return permission.RevokeAllRelations(ctx, s.perm, userID, driveID)
}
