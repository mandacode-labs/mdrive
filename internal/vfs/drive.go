package vfs

import (
	"context"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/user"
	"github.com/mandacode-labs/mdrive/internal/permission"
)

// CreateDrive creates a drive with storage config and root node. Grants owner in OpenFGA.
func (s *Service) CreateDrive(ctx context.Context, actorID string, name, description string, cfg drive.StorageConfig) (*drive.Drive, uuid.UUID, error) {
	d, rootID, err := s.drive.Create(ctx, name, &description, actorID, cfg)
	if err != nil {
		return nil, uuid.Nil, err
	}
	if s.perm != nil {
		_ = s.perm.Grant(ctx, actorID, permission.RelationOwner, permission.ObjectTypeDrive, d.ID())
	}
	return d, rootID, nil
}

// GetDrive returns a drive by private ID.
func (s *Service) GetDrive(ctx context.Context, id string) (*drive.Drive, error) {
	return s.drive.GetByID(ctx, id)
}

// GetDriveByPublicID returns a drive by public ID.
func (s *Service) GetDriveByPublicID(ctx context.Context, pubID string) (*drive.Drive, error) {
	return s.drive.GetByPublicID(ctx, pubID)
}

// GetDriveStorage returns the storage config for a drive.
func (s *Service) GetDriveStorage(ctx context.Context, driveID string) (*drive.Storage, error) {
	return s.drive.GetStorage(ctx, driveID)
}

// UpdateDrive updates drive fields.
func (s *Service) UpdateDrive(ctx context.Context, id string, name, description *string) (*drive.Drive, error) {
	return s.drive.Update(ctx, id, name, description)
}

// DeleteDrive removes a drive.
func (s *Service) DeleteDrive(ctx context.Context, id string) error {
	return s.drive.Delete(ctx, id)
}

// ListUserDrives returns all drives owned by actorID.
func (s *Service) ListUserDrives(ctx context.Context, actorID string) ([]*drive.Drive, error) {
	return s.drive.ListByOwner(ctx, actorID)
}

// UpsertUser creates or updates a user from OIDC claims.
func (s *Service) UpsertUser(ctx context.Context, cmd *user.CreateCommand) (*user.User, error) {
	return s.user.UpsertFromOIDC(ctx, cmd)
}

// GetUser returns a user by private ID.
func (s *Service) GetUser(ctx context.Context, id string) (*user.User, error) {
	return s.user.GetByID(ctx, id)
}
