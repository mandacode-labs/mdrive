package drive

import (
	"context"
	"time"

	"github.com/mandacode-labs/mdrive/ent"
	"github.com/mandacode-labs/mdrive/ent/drive"
	"github.com/mandacode-labs/mdrive/ent/storage"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/oklog/ulid/v2"
)

type entRepository struct {
	client *ent.Client
}

func NewRepository(client *ent.Client) Repository {
	return &entRepository{client: client}
}

var _ Repository = (*entRepository)(nil)

func (r *entRepository) Create(ctx context.Context, d *Drive) error {
	create := r.client.Drive.Create().
		SetID(d.id.String()).
		SetName(d.name).
		SetOwnerID(d.owner.String())
	if d.description != nil {
		create.SetDescription(*d.description)
	}
	_, err := create.Save(ctx)
	if err != nil {
		return errorx.Wrap(err, "drive: failed to create")
	}
	return nil
}

func (r *entRepository) Read(ctx context.Context, id ulid.ULID) (*Drive, error) {
	e, err := r.client.Drive.Query().Where(drive.IDEQ(id.String())).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, errorx.New(errorx.KindNotFound, "drive: not found")
		}
		return nil, errorx.Wrap(err, "drive: failed to load")
	}
	ownerID, err := ulid.Parse(e.OwnerID)
	if err != nil {
		return nil, errorx.Wrap(err, "drive: invalid owner id", errorx.KindInternal)
	}
	return Hydrate(
		ownerIDFromULID(e.ID),
		e.Name,
		e.Description,
		ownerID,
		e.DeletedAt,
		e.CreateTime,
		e.UpdateTime,
	), nil
}

func (r *entRepository) UpdateFields(ctx context.Context, id ulid.ULID, name string, description *string) (*Drive, error) {
	upd := r.client.Drive.UpdateOneID(id.String()).
		SetName(name).
		SetUpdateTime(time.Now())
	if description != nil {
		upd.SetDescription(*description)
	}
	e, err := upd.Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, errorx.New(errorx.KindNotFound, "drive: not found")
		}
		return nil, errorx.Wrap(err, "drive: failed to update", errorx.KindInternal)
	}
	ownerID, err := ulid.Parse(e.OwnerID)
	if err != nil {
		return nil, errorx.Wrap(err, "drive: invalid owner id", errorx.KindInternal)
	}
	return Hydrate(
		ownerIDFromULID(e.ID),
		e.Name,
		e.Description,
		ownerID,
		e.DeletedAt,
		e.CreateTime,
		e.UpdateTime,
	), nil
}

func (r *entRepository) SoftDelete(ctx context.Context, id ulid.ULID, at time.Time) error {
	_, err := r.client.Drive.UpdateOneID(id.String()).
		SetDeletedAt(at).
		SetUpdateTime(at).
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return errorx.New(errorx.KindNotFound, "drive: not found")
		}
		return errorx.Wrap(err, "drive: failed to soft-delete")
	}
	return nil
}

func (r *entRepository) Restore(ctx context.Context, id ulid.ULID) error {
	_, err := r.client.Drive.UpdateOneID(id.String()).
		ClearDeletedAt().
		SetUpdateTime(time.Now()).
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return errorx.New(errorx.KindNotFound, "drive: not found")
		}
		return errorx.Wrap(err, "drive: failed to restore")
	}
	return nil
}

func (r *entRepository) Destroy(ctx context.Context, id ulid.ULID) error {
	// Cascade on storage fires (set in schema). Superblock is
	// deleted via the superblock package's caller; the cascade
	// here only covers the storage row.
	if _, err := r.client.Storage.Delete().Where(storage.DriveIDEQ(id.String())).Exec(ctx); err != nil {
		return errorx.Wrap(err, "drive: failed to delete storage")
	}
	if err := r.client.Drive.DeleteOneID(id.String()).Exec(ctx); err != nil {
		return errorx.Wrap(err, "drive: failed to delete drive")
	}
	return nil
}

func (r *entRepository) ListByOwner(ctx context.Context, ownerID string) ([]*Drive, error) {
	rows, err := r.client.Drive.Query().
		Where(drive.OwnerIDEQ(ownerID)).
		Where(drive.DeletedAtIsNil()).
		All(ctx)
	if err != nil {
		return nil, errorx.Wrap(err, "drive: failed to list by owner")
	}
	out := make([]*Drive, 0, len(rows))
	for _, e := range rows {
		d, err := driveFromEnt(e)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

func (r *entRepository) ListDeleted(ctx context.Context, before time.Time, limit int) ([]*Drive, error) {
	rows, err := r.client.Drive.Query().
		Where(drive.DeletedAtNotNil()).
		Where(drive.DeletedAtLTE(before)).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, errorx.Wrap(err, "drive: failed to list deleted")
	}
	out := make([]*Drive, 0, len(rows))
	for _, e := range rows {
		d, err := driveFromEnt(e)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

func (r *entRepository) ReadStorage(ctx context.Context, driveID string) (*Storage, error) {
	e, err := r.client.Storage.Query().Where(storage.DriveIDEQ(driveID)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, errorx.New(errorx.KindNotFound, "drive: storage not found")
		}
		return nil, errorx.Wrap(err, "drive: failed to load storage")
	}
	return NewStorage(
		e.DriveID,
		e.Bucket,
		e.Endpoint,
		e.Region,
		e.AccessKey,
		e.SecretKey,
		e.UsePathStyle,
	), nil
}

func (r *entRepository) CreateStorage(ctx context.Context, s *Storage) error {
	create := r.client.Storage.Create().
		SetDriveID(s.DriveID()).
		SetProvider(storage.Provider(s.Provider().String())).
		SetBucket(s.Bucket()).
		SetRegion(s.Region()).
		SetAccessKey(s.AccessKey()).
		SetSecretKey(s.SecretKey()).
		SetUsePathStyle(s.UsePathStyle())
	if s.Endpoint() != nil {
		create.SetEndpoint(*s.Endpoint())
	}
	_, err := create.Save(ctx)
	if err != nil {
		return errorx.Wrap(err, "drive: failed to create storage")
	}
	return nil
}

// driveFromEnt is a shared loader.
func driveFromEnt(e *ent.Drive) (*Drive, error) {
	id, err := ulid.Parse(e.ID)
	if err != nil {
		return nil, errorx.Wrap(err, "drive: invalid id", errorx.KindInternal)
	}
	ownerID, err := ulid.Parse(e.OwnerID)
	if err != nil {
		return nil, errorx.Wrap(err, "drive: invalid owner id", errorx.KindInternal)
	}
	return Hydrate(id, e.Name, e.Description, ownerID, e.DeletedAt, e.CreateTime, e.UpdateTime), nil
}

// ownerIDFromULID converts ent's stored drive id (string ULID)
// back to ulid.ULID. Helper kept local since the ent Drive.ID
// is a string.
func ownerIDFromULID(id string) ulid.ULID {
	u, err := ulid.Parse(id)
	if err != nil {
		return ulid.ULID{}
	}
	return u
}
