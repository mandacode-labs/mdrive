package vfs

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/permission"
)

const (
	permView   = permission.PermissionView
	permEdit   = permission.PermissionEdit
	permDelete = permission.PermissionDelete
	permManage = permission.PermissionManage
)

// CreateDrive creates a drive with storage config and root node. Grants owner in OpenFGA.
func (s *Service) CreateDrive(ctx context.Context, actorID string, name, description string, cfg drive.StorageConfig) (*drive.Drive, uuid.UUID, error) {
	d, rootID, err := s.Drive.Create(ctx, name, &description, actorID, cfg)
	if err != nil {
		return nil, uuid.Nil, err
	}
	if s.Perm != nil {
		_ = s.Perm.Grant(ctx, actorID, permission.RelationOwner, permission.ObjectTypeDrive, d.ID())
	}
	return d, rootID, nil
}

// GetDrive returns a drive by private ID.
func (s *Service) GetDrive(ctx context.Context, actorID, id string) (*drive.Drive, error) {
	if err := s.checkAccess(ctx, actorID, permView, id); err != nil {
		return nil, err
	}
	return s.Drive.GetByID(ctx, id)
}

// GetDriveByPublicID returns a drive by public ID.
func (s *Service) GetDriveByPublicID(ctx context.Context, pubID string) (*drive.Drive, error) {
	return s.Drive.GetByPublicID(ctx, pubID)
}

// GetDriveStorage returns the storage config for a drive.
func (s *Service) GetDriveStorage(ctx context.Context, actorID, driveID string) (*drive.Storage, error) {
	if err := s.checkAccess(ctx, actorID, permView, driveID); err != nil {
		return nil, err
	}
	return s.Drive.GetStorage(ctx, driveID)
}

// UpdateDrive updates drive fields.
func (s *Service) UpdateDrive(ctx context.Context, actorID, id string, name, description *string) (*drive.Drive, error) {
	if err := s.checkAccess(ctx, actorID, permEdit, id); err != nil {
		return nil, err
	}
	return s.Drive.Update(ctx, id, name, description)
}

// DeleteDrive soft-deletes a drive.
func (s *Service) DeleteDrive(ctx context.Context, actorID, id string) error {
	if err := s.checkAccess(ctx, actorID, permDelete, id); err != nil {
		return err
	}
	return s.Drive.Delete(ctx, id)
}

// RestoreDrive reactivates a soft-deleted drive.
func (s *Service) RestoreDrive(ctx context.Context, actorID, id string) (*drive.Drive, error) {
	if err := s.checkAccess(ctx, actorID, permManage, id); err != nil {
		return nil, err
	}
	return s.Drive.Restore(ctx, id)
}

// ListUserDrives returns all active drives owned by actorID.
func (s *Service) ListUserDrives(ctx context.Context, actorID string) ([]*drive.Drive, error) {
	return s.Drive.ListByOwner(ctx, actorID)
}

// ListDeletedDrives returns soft-deleted drives (admin only).
func (s *Service) ListDeletedDrives(ctx context.Context) ([]*drive.Drive, error) {
	return s.Drive.ListDeleted(ctx, time.Now(), 1000)
}
