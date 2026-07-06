package vfs

import (
	"context"

	"github.com/google/uuid"
	"github.com/mandacode-labs/mdrive/ent"
	"github.com/mandacode-labs/mdrive/ent/drive"
	"github.com/mandacode-labs/mdrive/ent/superblock"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/oklog/ulid/v2"
)

// SuperOperation is the minimal contract vfs needs to access
// the root inode of a drive. Linux's super_operations interface
// is much broader (alloc_inode, write_inode, drop_inode, ...),
// but for path resolution we only need the root lookup.
//
// The data-access contract (and its ent-backed impl) lives in
// the same package — see SBRepository below.
type SuperOperation interface {
	GetRootNodeID(ctx context.Context, driveID ulid.ULID) (uuid.UUID, error)
}

// SBRepository is the data-access contract for superblocks.
// Implemented by the ent-backed sbrepo in this package.
type SBRepository interface {
	Create(ctx context.Context, sb *Superblock) error
	GetByDriveID(ctx context.Context, driveID ulid.ULID) (*Superblock, error)
	DeleteByDriveID(ctx context.Context, driveID ulid.ULID) error
}

type sbrepo struct {
	client *ent.Client
}

// NewSBRepository returns an ent-backed SBRepository.
func NewSBRepository(client *ent.Client) SBRepository {
	return &sbrepo{client: client}
}

var _ SBRepository = (*sbrepo)(nil)
var _ SuperOperation = (*sbrepo)(nil)

func (r *sbrepo) Create(ctx context.Context, sb *Superblock) error {
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

func (r *sbrepo) GetByDriveID(ctx context.Context, driveID ulid.ULID) (*Superblock, error) {
	e, err := r.client.Superblock.Query().
		Where(superblock.HasDriveWith(drive.IDEQ(driveID.String()))).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, errorx.New(errorx.KindNotFound, "superblock: not found")
		}
		return nil, errorx.Wrap(err, "superblock: failed to load")
	}
	driveEdge, err := e.QueryDrive().Only(ctx)
	if err != nil {
		return nil, errorx.Wrap(err, "superblock: failed to load drive edge", errorx.KindInternal)
	}
	parsedDriveID, err := ulid.Parse(driveEdge.ID)
	if err != nil {
		return nil, errorx.Wrap(err, "superblock: invalid drive id", errorx.KindInternal)
	}
	return HydrateSuperblock(e.ID, parsedDriveID, e.RootNodeID, e.CreateTime, e.UpdateTime), nil
}

func (r *sbrepo) DeleteByDriveID(ctx context.Context, driveID ulid.ULID) error {
	_, err := r.client.Superblock.Delete().
		Where(superblock.HasDriveWith(drive.IDEQ(driveID.String()))).
		Exec(ctx)
	if err != nil {
		return errorx.Wrap(err, "superblock: failed to delete")
	}
	return nil
}

// GetRootNodeID satisfies SuperOperation. vfs reaches the
// root inode of a drive via this method.
func (r *sbrepo) GetRootNodeID(ctx context.Context, driveID ulid.ULID) (uuid.UUID, error) {
	sb, err := r.GetByDriveID(ctx, driveID)
	if err != nil {
		return uuid.Nil, err
	}
	return sb.RootNodeID(), nil
}