package driveop

import (
	"context"
	"time"

	"github.com/mandacode-labs/mdrive/ent"
	entdrive "github.com/mandacode-labs/mdrive/ent/drive"
	"github.com/mandacode-labs/mdrive/internal/entx"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs"
	"github.com/oklog/ulid/v2"
)

// DriveRepository is the data-access contract for drives.
type DriveRepository interface {
	Read(ctx context.Context, id ulid.ULID) (*vfs.Drive, error)
	Write(ctx context.Context, d *vfs.Drive) error
	UpdateFields(ctx context.Context, id ulid.ULID, name string, description *string) (*vfs.Drive, error)
	SoftDelete(ctx context.Context, id ulid.ULID, at time.Time) error
	Restore(ctx context.Context, id ulid.ULID) error
	Destroy(ctx context.Context, id ulid.ULID) error
	ListByOwner(ctx context.Context, ownerID string) ([]*vfs.Drive, error)
	ListDeleted(ctx context.Context, before time.Time, limit int) ([]*vfs.Drive, error)
}

type driveRepo struct {
	client *ent.Client
}

func (r *driveRepo) Destroy(ctx context.Context, id ulid.ULID) error {
	client := r.client
	if tx, ok := entx.FromContext(ctx); ok {
		client = tx.Client()
	}
	err := client.Drive.DeleteOneID(id.String()).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return errorx.New(errorx.KindNotFound, "drive: not found")
		}
		return errorx.Wrap(err, "failed to delete drive", errorx.KindInternal)
	}
	return nil
}

func (r *driveRepo) Read(ctx context.Context, id ulid.ULID) (*vfs.Drive, error) {
	client := r.client
	if tx, ok := entx.FromContext(ctx); ok {
		client = tx.Client()
	}
	e, err := client.Drive.Query().Where(entdrive.IDEQ(id.String())).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, errorx.New(errorx.KindNotFound, "drive: not found")
		}
		return nil, errorx.Wrap(err, "failed to read drive", errorx.KindInternal)
	}
	return driveFromEnt(e)
}

func (r *driveRepo) Write(ctx context.Context, d *vfs.Drive) error {
	client := r.client
	if tx, ok := entx.FromContext(ctx); ok {
		client = tx.Client()
	}

	create := client.Drive.Create().
		SetID(d.ID().String()).
		SetName(d.Name()).
		SetRootNodeID(d.Root()).
		SetOwnerID(d.Owner().String()).
		SetCreateTime(d.CreatedAt()).
		SetUpdateTime(d.UpdatedAt())

	if d.Description() != nil {
		create.SetDescription(*d.Description())
	}
	if d.DeletedAt() != nil {
		create.SetDeletedAt(*d.DeletedAt())
	}

	err := create.OnConflict().
		UpdateNewValues().
		Exec(ctx)

	if err != nil {
		return errorx.Wrap(err, "failed to write drive", errorx.KindInternal)
	}

	return nil
}

func (r *driveRepo) UpdateFields(ctx context.Context, id ulid.ULID, name string, description *string) (*vfs.Drive, error) {
	client := r.client
	if tx, ok := entx.FromContext(ctx); ok {
		client = tx.Client()
	}
	upd := client.Drive.UpdateOneID(id.String()).
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
		return nil, errorx.Wrap(err, "failed to update drive", errorx.KindInternal)
	}
	return driveFromEnt(e)
}

func (r *driveRepo) SoftDelete(ctx context.Context, id ulid.ULID, at time.Time) error {
	client := r.client
	if tx, ok := entx.FromContext(ctx); ok {
		client = tx.Client()
	}
	_, err := client.Drive.UpdateOneID(id.String()).
		SetDeletedAt(at).
		SetUpdateTime(at).
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return errorx.New(errorx.KindNotFound, "drive: not found")
		}
		return errorx.Wrap(err, "failed to soft-delete drive", errorx.KindInternal)
	}
	return nil
}

func (r *driveRepo) Restore(ctx context.Context, id ulid.ULID) error {
	client := r.client
	if tx, ok := entx.FromContext(ctx); ok {
		client = tx.Client()
	}
	_, err := client.Drive.UpdateOneID(id.String()).
		ClearDeletedAt().
		SetUpdateTime(time.Now()).
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return errorx.New(errorx.KindNotFound, "drive: not found")
		}
		return errorx.Wrap(err, "failed to restore drive", errorx.KindInternal)
	}
	return nil
}

func (r *driveRepo) ListByOwner(ctx context.Context, ownerID string) ([]*vfs.Drive, error) {
	client := r.client
	if tx, ok := entx.FromContext(ctx); ok {
		client = tx.Client()
	}
	rows, err := client.Drive.Query().
		Where(entdrive.OwnerIDEQ(ownerID)).
		Where(entdrive.DeletedAtIsNil()).
		All(ctx)
	if err != nil {
		return nil, errorx.Wrap(err, "failed to list drives by owner", errorx.KindInternal)
	}
	out := make([]*vfs.Drive, 0, len(rows))
	for _, e := range rows {
		d, err := driveFromEnt(e)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

func (r *driveRepo) ListDeleted(ctx context.Context, before time.Time, limit int) ([]*vfs.Drive, error) {
	client := r.client
	if tx, ok := entx.FromContext(ctx); ok {
		client = tx.Client()
	}
	rows, err := client.Drive.Query().
		Where(entdrive.DeletedAtNotNil()).
		Where(entdrive.DeletedAtLTE(before)).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, errorx.Wrap(err, "failed to list deleted drives", errorx.KindInternal)
	}
	out := make([]*vfs.Drive, 0, len(rows))
	for _, e := range rows {
		d, err := driveFromEnt(e)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

func NewDriveRepository(client *ent.Client) DriveRepository {
	return &driveRepo{client: client}
}

var _ DriveRepository = (*driveRepo)(nil)

func driveFromEnt(e *ent.Drive) (*vfs.Drive, error) {
	if e == nil {
		return nil, errorx.New(errorx.KindNotFound, "drive: not found")
	}

	id, err := ulid.Parse(e.ID)
	if err != nil {
		return nil, errorx.Wrap(err, "failed to parse drive ID", errorx.KindInvalidArgument)
	}
	ownerID, err := ulid.Parse(e.OwnerID)
	if err != nil {
		return nil, errorx.Wrap(err, "failed to parse owner ID", errorx.KindInvalidArgument)
	}

	return vfs.HydrateDrive(
		id,
		e.Name,
		e.Description,
		e.RootNodeID,
		ownerID,
		e.DeletedAt,
		e.CreateTime,
		e.UpdateTime,
	), nil
}
