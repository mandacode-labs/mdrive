package driveop

import (
	"context"

	"github.com/mandacode-labs/mdrive/ent"
	entdrive "github.com/mandacode-labs/mdrive/ent/drive"
	"github.com/mandacode-labs/mdrive/internal/entx"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs"
	"github.com/oklog/ulid/v2"
)

// BlockStorage is the data-access contract for nodes.
type BlockStorage interface {
	Read(ctx context.Context, id ulid.ULID) (*vfs.Drive, error)
	Write(ctx context.Context, n *vfs.Drive) error
	Destroy(ctx context.Context, id ulid.ULID) error
}

type blockStorage struct {
	client *ent.Client
}

// Destroy implements [BlockStorage].
func (bs *blockStorage) Destroy(ctx context.Context, id ulid.ULID) error {
	client := bs.client
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

// Read implements [BlockStorage].
func (bs *blockStorage) Read(ctx context.Context, id ulid.ULID) (*vfs.Drive, error) {
	client := bs.client
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
	return fromEnt(e)
}

// Write implements [BlockStorage].
func (bs *blockStorage) Write(ctx context.Context, d *vfs.Drive) error {
	client := bs.client
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

func NewBlockStorage(client *ent.Client) BlockStorage {
	return &blockStorage{client: client}
}

func fromEnt(e *ent.Drive) (*vfs.Drive, error) {
	if e == nil {
		return nil, errorx.New(errorx.KindNotFound, "node: not found")
	}

	id, err := ulid.Parse(e.ID)
	if err != nil {
		return nil, errorx.Wrap(err, "failed to parse drive ID", errorx.KindInvalidArgument)
	}
	ownerID, err := ulid.Parse(e.OwnerID)

	d := vfs.HydrateDrive(
		id,
		e.Name,
		e.Description,
		e.RootNodeID,
		ownerID,
		e.DeletedAt,
		e.CreateTime,
		e.UpdateTime,
	)

	return d, nil
}
