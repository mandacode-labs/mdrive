package storageop

import (
	"context"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
	"github.com/mandacode-labs/mdrive/ent"
	"github.com/mandacode-labs/mdrive/ent/storage"
	"github.com/mandacode-labs/mdrive/ent/superblock"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
)

// StorageRepository is the data-access contract for fs.Storage.
// The lookup is keyed by superblock_id; the repo internally
// joins superblock → drive_id → storage row.
type StorageRepository interface {
	GetBySuperblock(ctx context.Context, superblockID uuid.UUID) (*fs.Storage, error)
}

// entStorageRepository is the ent-backed impl.
type entStorageRepository struct {
	client *ent.Client
}

func NewStorageRepository(client *ent.Client) StorageRepository {
	return &entStorageRepository{client: client}
}

// GetBySuperblock looks up the drive_id from the superblock,
// then loads the storage row for that drive. Returns nil +
// nil when no storage is configured (caller falls back to
// default IRSA).
func (r *entStorageRepository) GetBySuperblock(ctx context.Context, superblockID uuid.UUID) (*fs.Storage, error) {
	sb, err := r.client.Superblock.Query().
		Where(superblock.IDEQ(superblockID)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, errorx.Wrap(err, "storageop: lookup superblock")
	}

	driveID, ulidErr := ulid.Parse(sb.DriveID)
	if ulidErr != nil {
		return nil, errorx.Wrap(ulidErr, "storageop: parse drive id from superblock")
	}

	s, err := r.client.Storage.Query().
		Where(storage.DriveIDEQ(sb.DriveID)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, errorx.Wrap(err, "storageop: lookup storage")
	}

	return fromEnt(s, driveID), nil
}

func fromEnt(e *ent.Storage, driveID ulid.ULID) *fs.Storage {
	return fs.NewStorage(
		driveID,
		string(e.Provider),
		e.Bucket,
		e.Region,
		e.Endpoint,
		e.AccessKey,
		e.EncryptedSecretKey,
		e.UsePathStyle,
	)
}