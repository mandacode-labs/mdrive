package superop

import (
	"context"

	"github.com/google/uuid"
	"github.com/mandacode-labs/mdrive/ent"
	"github.com/mandacode-labs/mdrive/ent/superblock"
	"github.com/mandacode-labs/mdrive/internal/entx"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs"
	"github.com/oklog/ulid/v2"
)

type Repository interface {
	Create(ctx context.Context, sb *vfs.Superblock) error
	Read(ctx context.Context, id uuid.UUID) (*vfs.Superblock, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type entRepository struct {
	client *ent.Client
}

// Create implements [Repository].
func (e *entRepository) Create(ctx context.Context, sb *vfs.Superblock) error {
	client := e.client
	if tx, ok := entx.FromContext(ctx); ok {
		client = tx.Client()
	}
	if sb == nil {
		return errorx.New(errorx.KindInvalidArgument, "superblock: cannot create nil superblock")
	}

	// Create the superblock entity in the database
	// If sb is already present in the database, this will return an error
	_, err := client.Superblock.Create().
		SetID(sb.ID()).
		SetDriveID(sb.DriveID().String()).
		SetRootNodeID(sb.RootNodeID()).
		SetCreateTime(sb.CreatedAt()).
		SetUpdateTime(sb.UpdatedAt()).
		Save(ctx)
	if err != nil {
		return errorx.Wrap(err, "superblock: failed to create superblock")
	}
	return nil
}

// Delete implements [Repository].
func (e *entRepository) Delete(ctx context.Context, id uuid.UUID) error {
	client := e.client
	if tx, ok := entx.FromContext(ctx); ok {
		client = tx.Client()
	}
	err := client.Superblock.DeleteOneID(id).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return errorx.New(errorx.KindNotFound, "superblock: not found")
		}
		return errorx.Wrap(err, "superblock: failed to delete superblock")
	}
	return nil
}

// Read implements [Repository].
func (e *entRepository) Read(ctx context.Context, id uuid.UUID) (*vfs.Superblock, error) {
	client := e.client
	if tx, ok := entx.FromContext(ctx); ok {
		client = tx.Client()
	}
	sb, err := client.Superblock.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, errorx.New(errorx.KindNotFound, "superblock: not found")
		}
		return nil, errorx.Wrap(err, "superblock: failed to read superblock")
	}

	return fromEnt(sb)
}

func NewRepository(client *ent.Client) Repository {
	return &entRepository{client: client}
}

func fromEnt(e *ent.Superblock) (*vfs.Superblock, error) {
	driveID, err := ulid.Parse(e.DriveID)
	if err != nil {
		return nil, errorx.Wrap(err, "superblock: failed to parse drive ID")
	}
	return vfs.HydrateSuperblock(
		e.ID,
		driveID,
		e.RootNodeID,
		e.CreateTime,
		e.UpdateTime,
	), nil
}
