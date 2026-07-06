package driveop

import (
	"context"
	"time"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs"
	"github.com/oklog/ulid/v2"
)

// CreateDrive implements [DriveOperation]. Validates the owner,
// creates the drive's root directory inode, encrypts the storage
// secret, and persists Drive + Storage in a single transaction.
// The returned Drive has RootNodeID set and is ready for use.
func (d *driveOperation) CreateDrive(
	ctx context.Context,
	ownerID string,
	name string,
	description string,
	storage *vfs.Storage,
) (*vfs.Drive, error) {
	if name == "" {
		return nil, errorx.New(errorx.KindInvalidArgument, "driveop: name is required")
	}
	if ownerID == "" {
		return nil, errorx.New(errorx.KindInvalidArgument, "driveop: owner_id is required")
	}
	if storage == nil {
		return nil, errorx.New(errorx.KindInvalidArgument, "driveop: storage is required")
	}

	exists, err := d.owner.Exist(ctx, ownerID)
	if err != nil {
		return nil, errorx.Wrap(err, "driveop: owner check failed", errorx.KindUnavailable)
	}
	if !exists {
		return nil, errorx.New(errorx.KindPermissionDenied, "driveop: owner does not exist")
	}

	root, err := d.root.CreateRootDirectory(ctx)
	if err != nil {
		return nil, errorx.Wrap(err, "driveop: failed to create root directory", errorx.KindUnavailable)
	}

	ownerULID, err := ulid.Parse(ownerID)
	if err != nil {
		return nil, errorx.Wrap(err, "driveop: invalid owner id", errorx.KindInvalidArgument)
	}
	driveID := ulid.Make()

	var descPtr *string
	if description != "" {
		descPtr = &description
	}

	encryptedSecret, err := d.cipher.Encrypt([]byte(storage.SecretKey()))
	if err != nil {
		return nil, errorx.Wrap(err, "driveop: failed to encrypt storage secret", errorx.KindInternal)
	}

	drive := vfs.NewDrive(driveID, name, root.ID(), ownerULID)
	drive.SetDescription(descPtr)

	now := time.Now()
	_ = now

	var created *vfs.Drive
	err = d.tm.WithTx(ctx, func(ctx context.Context) error {
		if err := d.repo.Write(ctx, drive); err != nil {
			return errorx.Wrap(err, "driveop: failed to insert drive", errorx.KindInternal)
		}
		storageRow := vfs.NewStorage(
			driveID.String(),
			storage.Bucket(),
			storage.Endpoint(),
			storage.Region(),
			storage.AccessKey(),
			string(encryptedSecret),
			storage.UsePathStyle(),
		)
		if err := d.storage.Write(ctx, storageRow); err != nil {
			return errorx.Wrap(err, "driveop: failed to insert storage", errorx.KindInternal)
		}
		updated, err := d.repo.UpdateFields(ctx, drive.ID(), drive.Name(), drive.Description())
		if err != nil {
			return errorx.Wrap(err, "driveop: failed to refresh drive", errorx.KindInternal)
		}
		created = updated
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}
