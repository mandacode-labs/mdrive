package storageop

import (
	"context"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
	"github.com/mandacode-labs/mdrive/ent"
	"github.com/mandacode-labs/mdrive/ent/storage"
	"github.com/mandacode-labs/mdrive/ent/superblock"
	"github.com/mandacode-labs/mdrive/internal/crypto"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
)

// StorageRepository is the data-access contract for fs.Storage.
// The lookup is keyed by superblock_id; the repo internally
// joins superblock → drive_id → storage row and decrypts the
// secret key on read.
type StorageRepository interface {
	GetBySuperblock(ctx context.Context, superblockID uuid.UUID) (*fs.Storage, error)
}

// entStorageRepository is the ent-backed impl.
type entStorageRepository struct {
	client   *ent.Client
	decryptor crypto.Decryptor
}

func NewStorageRepository(client *ent.Client, decryptor crypto.Decryptor) StorageRepository {
	return &entStorageRepository{client: client, decryptor: decryptor}
}

// GetBySuperblock looks up the drive_id from the superblock,
// then loads the storage row for that drive. Returns nil +
// nil when no storage is configured (caller falls back to
// app-level default).
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

	storage, err := fromEnt(r.decryptor, driveID, s)
	if err != nil {
		return nil, errorx.Wrap(err, "storageop: decrypt storage")
	}
	return storage, nil
}

// fromEnt converts an ent Storage row to fs.Storage. The
// encrypted_secret_key is decrypted here (decryptor may be
// nil if the row has no secret — e.g. IRSA / public bucket).
func fromEnt(d crypto.Decryptor, driveID ulid.ULID, e *ent.Storage) (*fs.Storage, error) {
	secretKey := ""
	if e.EncryptedSecretKey != nil && *e.EncryptedSecretKey != "" {
		if d == nil {
			// encrypted_secret_key is set but no decryptor —
			// treat as plaintext in dev; in prod this is a
			// configuration error caught at startup.
			secretKey = *e.EncryptedSecretKey
		} else {
			dec, err := d.Decrypt([]byte(*e.EncryptedSecretKey))
			if err != nil {
				return nil, err
			}
			secretKey = string(dec)
		}
	}
	return fs.NewStorage(
		driveID,
		string(e.Provider),
		strDeref(e.Bucket),
		strDeref(e.Region),
		&e.Endpoint,
		strDeref(e.AccessKey),
		secretKey,
		boolDeref(e.UsePathStyle),
	), nil
}

func strDeref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func boolDeref(p *bool) bool {
	if p == nil {
		return false
	}
	return *p
}