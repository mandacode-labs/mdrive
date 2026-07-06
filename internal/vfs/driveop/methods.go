package driveop

import (
	"context"
	"time"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs"
	"github.com/mandacode-labs/mdrive/internal/vfs/permission"
)

// GetDrive implements [DriveOperation].
func (d *driveOperation) GetDrive(ctx context.Context, driveID string) (*vfs.Drive, error) {
	id, err := driveIDFromString(driveID)
	if err != nil {
		return nil, err
	}
	if err := d.requirePerm(ctx, permission.ActionView, id); err != nil {
		return nil, err
	}
	drive, err := d.repo.Read(ctx, id)
	if err != nil {
		return nil, err
	}
	if drive == nil {
		return nil, errorx.New(errorx.KindNotFound, "driveop: drive not found")
	}
	return drive, nil
}

// GetDriveStorage implements [DriveOperation]. Returns the
// decrypted S3/MinIO backend configuration for the given drive.
func (d *driveOperation) GetDriveStorage(ctx context.Context, driveID string) (*vfs.Storage, error) {
	id, err := driveIDFromString(driveID)
	if err != nil {
		return nil, err
	}
	if err := d.requirePerm(ctx, permission.ActionView, id); err != nil {
		return nil, err
	}
	storage, err := d.storage.Read(ctx, driveID)
	if err != nil {
		return nil, err
	}
	if storage == nil {
		return nil, errorx.New(errorx.KindNotFound, "driveop: storage not found")
	}
	plain, err := d.cipher.Decrypt([]byte(storage.SecretKey()))
	if err != nil {
		return nil, errorx.Wrap(err, "driveop: failed to decrypt storage secret", errorx.KindInternal)
	}
	return vfs.NewStorage(
		storage.DriveID(),
		storage.Bucket(),
		storage.Endpoint(),
		storage.Region(),
		storage.AccessKey(),
		string(plain),
		storage.UsePathStyle(),
	), nil
}

// UpdateDrive implements [DriveOperation].
func (d *driveOperation) UpdateDrive(ctx context.Context, driveID, name, description string) (*vfs.Drive, error) {
	id, err := driveIDFromString(driveID)
	if err != nil {
		return nil, err
	}
	if err := d.requirePerm(ctx, permission.ActionEdit, id); err != nil {
		return nil, err
	}
	existing, err := d.repo.Read(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errorx.New(errorx.KindNotFound, "driveop: drive not found")
	}
	newName := existing.Name()
	if name != "" {
		newName = name
	}
	var descPtr *string
	if description != "" {
		descPtr = &description
	}
	updated, err := d.repo.UpdateFields(ctx, id, newName, descPtr)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, errorx.New(errorx.KindNotFound, "driveop: drive not found after update")
	}
	return updated, nil
}

// SoftDeleteDrive implements [DriveOperation]. Sets DeletedAt;
// reversible via RestoreDrive.
func (d *driveOperation) SoftDeleteDrive(ctx context.Context, driveID string) error {
	id, err := driveIDFromString(driveID)
	if err != nil {
		return err
	}
	if err := d.requirePerm(ctx, permission.ActionEdit, id); err != nil {
		return err
	}
	if _, err := d.repo.Read(ctx, id); err != nil {
		return err
	}
	return d.repo.SoftDelete(ctx, id, time.Now())
}

// RestoreDrive implements [DriveOperation].
func (d *driveOperation) RestoreDrive(ctx context.Context, driveID string) (*vfs.Drive, error) {
	id, err := driveIDFromString(driveID)
	if err != nil {
		return nil, err
	}
	if err := d.requirePerm(ctx, permission.ActionEdit, id); err != nil {
		return nil, err
	}
	existing, err := d.repo.Read(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errorx.New(errorx.KindNotFound, "driveop: drive not found")
	}
	if existing.DeletedAt() == nil {
		return nil, errorx.New(errorx.KindFailedPrecondition, "driveop: drive is not deleted")
	}
	if err := d.repo.Restore(ctx, id); err != nil {
		return nil, err
	}
	return d.repo.Read(ctx, id)
}

// PurgeDrive implements [DriveOperation]. Hard-deletes the Drive
// row; the schema-level Cascade on Storage removes the storage
// row automatically.
func (d *driveOperation) PurgeDrive(ctx context.Context, driveID string) error {
	id, err := driveIDFromString(driveID)
	if err != nil {
		return err
	}
	if err := d.requirePerm(ctx, permission.ActionDelete, id); err != nil {
		return err
	}
	return d.repo.Destroy(ctx, id)
}

// ListDrivesByOwner implements [DriveOperation]. No permission
// enforcement — caller (vfs high-level wrapper) decides.
func (d *driveOperation) ListDrivesByOwner(ctx context.Context, ownerID string) ([]*vfs.Drive, error) {
	if ownerID == "" {
		return nil, errorx.New(errorx.KindInvalidArgument, "driveop: owner_id is required")
	}
	return d.repo.ListByOwner(ctx, ownerID)
}

// ListDeletedDrives implements [DriveOperation]. No permission
// enforcement — caller (vfs high-level wrapper) decides.
func (d *driveOperation) ListDeletedDrives(ctx context.Context, before time.Time, limit int) ([]*vfs.Drive, error) {
	if limit <= 0 {
		limit = 1000
	}
	return d.repo.ListDeleted(ctx, before, limit)
}
