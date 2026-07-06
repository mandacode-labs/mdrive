package drive

import (
	"context"
	"time"

	"github.com/oklog/ulid/v2"
)

// Operation is the public surface for drive lifecycle. Handlers
// call into this; vfs does not depend on Operation (it uses
// superblock.Operation for root access).
type Operation interface {
	Create(ctx context.Context, ownerID string, name string, description string, storage *Storage) (*Drive, error)
	Get(ctx context.Context, driveID string) (*Drive, error)
	GetStorage(ctx context.Context, driveID string) (*Storage, error)
	Update(ctx context.Context, driveID string, name string, description string) (*Drive, error)
	SoftDelete(ctx context.Context, driveID string) error
	Restore(ctx context.Context, driveID string) (*Drive, error)
	Purge(ctx context.Context, driveID string) error
	ListByOwner(ctx context.Context, ownerID string) ([]*Drive, error)
	ListDeleted(ctx context.Context, before time.Time, limit int) ([]*Drive, error)
}

type operation struct {
	repo Repository
}

// NewOperation wires the canonical impl.
func NewOperation(repo Repository) Operation {
	return &operation{repo: repo}
}

var _ Operation = (*operation)(nil)

func (o *operation) Create(ctx context.Context, ownerID string, name string, description string, storage *Storage) (*Drive, error) {
	if name == "" {
		return nil, errorxKindInvalidArgument("drive: name is required")
	}
	if ownerID == "" {
		return nil, errorxKindInvalidArgument("drive: owner_id is required")
	}
	if storage == nil {
		return nil, errorxKindInvalidArgument("drive: storage is required")
	}
	ownerULID, err := ulid.Parse(ownerID)
	if err != nil {
		return nil, errorxKindInvalidArgument("drive: invalid owner id")
	}
	id := ulid.Make()

	var descPtr *string
	if description != "" {
		descPtr = &description
	}

	d := New(id, name, ownerULID)
	d.SetDescription(descPtr)

	if err := o.repo.Create(ctx, d); err != nil {
		return nil, err
	}

	storageRow := NewStorage(
		id.String(),
		storage.Bucket(),
		storage.Endpoint(),
		storage.Region(),
		storage.AccessKey(),
		storage.SecretKey(),
		storage.UsePathStyle(),
	)
	if err := o.repo.CreateStorage(ctx, storageRow); err != nil {
		return nil, err
	}

	created, err := o.repo.UpdateFields(ctx, id, name, descPtr)
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (o *operation) Get(ctx context.Context, driveID string) (*Drive, error) {
	id, err := parseDriveID(driveID)
	if err != nil {
		return nil, err
	}
	return o.repo.Read(ctx, id)
}

func (o *operation) GetStorage(ctx context.Context, driveID string) (*Storage, error) {
	return o.repo.ReadStorage(ctx, driveID)
}

func (o *operation) Update(ctx context.Context, driveID string, name string, description string) (*Drive, error) {
	id, err := parseDriveID(driveID)
	if err != nil {
		return nil, err
	}
	existing, err := o.repo.Read(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errorxKindNotFound("drive: not found")
	}
	newName := existing.Name()
	if name != "" {
		newName = name
	}
	var descPtr *string
	if description != "" {
		descPtr = &description
	}
	return o.repo.UpdateFields(ctx, id, newName, descPtr)
}

func (o *operation) SoftDelete(ctx context.Context, driveID string) error {
	id, err := parseDriveID(driveID)
	if err != nil {
		return err
	}
	if _, err := o.repo.Read(ctx, id); err != nil {
		return err
	}
	return o.repo.SoftDelete(ctx, id, time.Now())
}

func (o *operation) Restore(ctx context.Context, driveID string) (*Drive, error) {
	id, err := parseDriveID(driveID)
	if err != nil {
		return nil, err
	}
	existing, err := o.repo.Read(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.DeletedAt() == nil {
		return nil, errorxKindFailedPrecondition("drive: not deleted")
	}
	if err := o.repo.Restore(ctx, id); err != nil {
		return nil, err
	}
	return o.repo.Read(ctx, id)
}

func (o *operation) Purge(ctx context.Context, driveID string) error {
	id, err := parseDriveID(driveID)
	if err != nil {
		return err
	}
	return o.repo.Destroy(ctx, id)
}

func (o *operation) ListByOwner(ctx context.Context, ownerID string) ([]*Drive, error) {
	if ownerID == "" {
		return nil, errorxKindInvalidArgument("drive: owner_id is required")
	}
	return o.repo.ListByOwner(ctx, ownerID)
}

func (o *operation) ListDeleted(ctx context.Context, before time.Time, limit int) ([]*Drive, error) {
	if limit <= 0 {
		limit = 1000
	}
	return o.repo.ListDeleted(ctx, before, limit)
}