package superblock

import (
	"context"

	"github.com/mandacode-labs/mdrive/ent"
	"github.com/mandacode-labs/mdrive/ent/drive"
	"github.com/mandacode-labs/mdrive/ent/superblock"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/oklog/ulid/v2"
)

type entRepository struct {
	client *ent.Client
}

// NewRepository returns an ent-backed Repository.
func NewRepository(client *ent.Client) Repository {
	return &entRepository{client: client}
}

var _ Repository = (*entRepository)(nil)

func (r *entRepository) Create(ctx context.Context, sb *Superblock) error {
	_, err := r.client.Superblock.Create().
		SetID(sb.id).
		SetDriveID(sb.driveID.String()).
		SetRootNodeID(sb.rootNodeID).
		Save(ctx)
	if err != nil {
		return errorx.Wrap(err, "superblock: failed to create")
	}
	return nil
}

func (r *entRepository) GetByDriveID(ctx context.Context, driveID ulid.ULID) (*Superblock, error) {
	e, err := r.client.Superblock.Query().
		Where(superblock.HasDriveWith(drive.IDEQ(driveID.String()))).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, errorx.New(errorx.KindNotFound, "superblock: not found")
		}
		return nil, errorx.Wrap(err, "superblock: failed to load")
	}
	// drive_id lives on the edge; load it.
	driveEdge, err := e.QueryDrive().Only(ctx)
	if err != nil {
		return nil, errorx.Wrap(err, "superblock: failed to load drive edge", errorx.KindInternal)
	}
	parsedDriveID, err := ulid.Parse(driveEdge.ID)
	if err != nil {
		return nil, errorx.Wrap(err, "superblock: invalid drive id", errorx.KindInternal)
	}
	return Hydrate(e.ID, parsedDriveID, e.RootNodeID, e.CreateTime, e.UpdateTime), nil
}

func (r *entRepository) DeleteByDriveID(ctx context.Context, driveID ulid.ULID) error {
	_, err := r.client.Superblock.Delete().
		Where(superblock.HasDriveWith(drive.IDEQ(driveID.String()))).
		Exec(ctx)
	if err != nil {
		return errorx.Wrap(err, "superblock: failed to delete")
	}
	return nil
}