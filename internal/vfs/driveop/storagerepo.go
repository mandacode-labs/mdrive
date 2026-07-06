package driveop

import (
	"context"

	"github.com/mandacode-labs/mdrive/ent"
	entstorage "github.com/mandacode-labs/mdrive/ent/storage"
	"github.com/mandacode-labs/mdrive/internal/entx"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs"
)

// StorageRepository is the data-access contract for per-drive
// S3/MinIO backend configuration. Storage is a 1:1 child of Drive;
// cascade delete is enforced at the schema level.
type StorageRepository interface {
	Read(ctx context.Context, driveID string) (*vfs.Storage, error)
	Write(ctx context.Context, s *vfs.Storage) error
	Destroy(ctx context.Context, driveID string) error
}

type storageRepo struct {
	client *ent.Client
}

func (r *storageRepo) Read(ctx context.Context, driveID string) (*vfs.Storage, error) {
	client := r.client
	if tx, ok := entx.FromContext(ctx); ok {
		client = tx.Client()
	}
	e, err := client.Storage.Query().Where(entstorage.DriveIDEQ(driveID)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, errorx.New(errorx.KindNotFound, "storage: not found")
		}
		return nil, errorx.Wrap(err, "failed to read storage", errorx.KindInternal)
	}
	return storageFromEnt(e)
}

func (r *storageRepo) Write(ctx context.Context, s *vfs.Storage) error {
	client := r.client
	if tx, ok := entx.FromContext(ctx); ok {
		client = tx.Client()
	}

	create := client.Storage.Create().
		SetDriveID(s.DriveID()).
		SetProvider(entstorage.Provider(s.Provider().String())).
		SetBucket(s.Bucket()).
		SetRegion(s.Region()).
		SetAccessKey(s.AccessKey()).
		SetSecretKey(s.SecretKey()).
		SetUsePathStyle(s.UsePathStyle())

	if s.Endpoint() != nil {
		create.SetEndpoint(*s.Endpoint())
	}

	err := create.OnConflict().
		UpdateNewValues().
		Exec(ctx)

	if err != nil {
		return errorx.Wrap(err, "failed to write storage", errorx.KindInternal)
	}
	return nil
}

func (r *storageRepo) Destroy(ctx context.Context, driveID string) error {
	client := r.client
	if tx, ok := entx.FromContext(ctx); ok {
		client = tx.Client()
	}
	_, err := client.Storage.Delete().Where(entstorage.DriveIDEQ(driveID)).Exec(ctx)
	if err != nil {
		return errorx.Wrap(err, "failed to delete storage", errorx.KindInternal)
	}
	return nil
}

func NewStorageRepository(client *ent.Client) StorageRepository {
	return &storageRepo{client: client}
}

var _ StorageRepository = (*storageRepo)(nil)

func storageFromEnt(e *ent.Storage) (*vfs.Storage, error) {
	if e == nil {
		return nil, errorx.New(errorx.KindNotFound, "storage: not found")
	}
	return vfs.NewStorage(
		e.DriveID,
		e.Bucket,
		e.Endpoint,
		e.Region,
		e.AccessKey,
		e.SecretKey,
		e.UsePathStyle,
	), nil
}
