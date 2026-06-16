package vfs

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/permission"
)

// Drive operations on the vfs Service.

// CreateDrive creates a drive (with storage config) and its root directory node.
// Returns the drive and the root node's ID. Grants the owner relation in OpenFGA.
func (s *Service) CreateDrive(ctx context.Context, actorID string, cfg drive.StorageConfig, name, description string) (*drive.Drive, uuid.UUID, error) {
	if name == "" {
		return nil, uuid.Nil, drive.ErrInvalidName
	}
	if actorID == "" {
		return nil, uuid.Nil, fmt.Errorf("vfs: actor_id is required")
	}

	// Verify owner exists.
	ok, err := s.user.GetByID(ctx, actorID)
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("vfs: check owner: %w", err)
	}
	if ok == nil {
		return nil, uuid.Nil, fmt.Errorf("vfs: owner not found")
	}

	id := ulid.Make().String()
	now := time.Now()
	d := drive.NewDrive(id, id, name, &description, drive.ProviderS3, actorID, nil, now, now)

	st := drive.NewStorage(
		id, cfg.Bucket, cfg.Endpoint, cfg.Region,
		cfg.AccessKey, cfg.SecretKey, cfg.UsePathStyle,
	)

	if err := s.drive.Create(ctx, d, st); err != nil {
		return nil, uuid.Nil, fmt.Errorf("vfs: create drive: %w", err)
	}

	// Create root directory node.
	root, err := s.CreateDirectory(ctx)
	if err != nil {
		_ = s.drive.Delete(ctx, id)
		return nil, uuid.Nil, fmt.Errorf("vfs: create root node: %w", err)
	}
	d.SetRootNodeID(root.ID())

	updated, err := s.drive.Update(ctx, d)
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("vfs: update drive with root: %w", err)
	}

	// Grant owner role in OpenFGA.
	if err := s.perm.Grant(ctx, actorID, permission.RelationOwner, permission.ObjectTypeDrive, id); err != nil {
		// Non-fatal: drive is still usable, but the permission will be missing.
		// A compensating action (retry / alert) would be added in production.
	}

	return updated, root.ID(), nil
}

// GetDrive returns a drive by its private ID.
func (s *Service) GetDrive(ctx context.Context, id string) (*drive.Drive, error) {
	d, err := s.drive.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, drive.ErrNotFound
	}
	return d, nil
}

// GetDriveByPublicID returns a drive by its public ID.
func (s *Service) GetDriveByPublicID(ctx context.Context, publicID string) (*drive.Drive, error) {
	d, err := s.drive.GetByPublicID(ctx, publicID)
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, drive.ErrNotFound
	}
	return d, nil
}

// GetDriveStorage returns the storage config for a drive.
func (s *Service) GetDriveStorage(ctx context.Context, driveID string) (*drive.Storage, error) {
	st, err := s.drive.GetStorage(ctx, driveID)
	if err != nil {
		return nil, err
	}
	if st == nil {
		return nil, drive.ErrNotFound
	}
	return st, nil
}

// UpdateDrive updates mutable drive fields.
func (s *Service) UpdateDrive(ctx context.Context, id string, name, description *string) (*drive.Drive, error) {
	d, err := s.drive.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, drive.ErrNotFound
	}
	newName := d.Name()
	if name != nil {
		newName = *name
	}
	newDesc := d.Description()
	if description != nil {
		newDesc = description
	}
	updated := drive.NewDrive(
		d.ID(), d.PublicID(), newName, newDesc,
		d.Provider(), d.OwnerID(), d.RootNodeID(),
		d.CreatedAt(), time.Now(),
	)
	return s.drive.Update(ctx, updated)
}

// DeleteDrive removes a drive.
// Caller must first walk the tree to delete all nodes and clean up S3.
func (s *Service) DeleteDrive(ctx context.Context, id string) error {
	d, err := s.drive.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if d == nil {
		return drive.ErrNotFound
	}
	return s.drive.Delete(ctx, d.ID())
}

// ListUserDrives returns all drives owned by actorID.
func (s *Service) ListUserDrives(ctx context.Context, actorID string) ([]*drive.Drive, error) {
	return s.drive.FindByOwner(ctx, actorID)
}

// GrantDriveAccess grants a relation (editor, viewer) to a user on a drive.
func (s *Service) GrantDriveAccess(ctx context.Context, actorID, userID, driveID, relation string) error {
	return s.perm.Grant(ctx, userID, relation, permission.ObjectTypeDrive, driveID)
}

// RevokeDriveAccess revokes all relations for a user on a drive.
func (s *Service) RevokeDriveAccess(ctx context.Context, actorID, userID, driveID string) error {
	return permission.RevokeAllRelations(ctx, s.perm, userID, driveID)
}
