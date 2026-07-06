package superblock

import (
	"context"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
)

// Operation is the public surface. vfs uses GetRootNodeID;
// callers that want full lifecycle (Create on drive creation,
// Delete on drive purge) use Create / Delete.
type Operation interface {
	Create(ctx context.Context, driveID ulid.ULID, rootNodeID uuid.UUID) (*Superblock, error)
	GetRootNodeID(ctx context.Context, driveID ulid.ULID) (uuid.UUID, error)
	DeleteByDriveID(ctx context.Context, driveID ulid.ULID) error
}

type operation struct {
	repo Repository
}

// NewOperation wires the canonical impl.
func NewOperation(repo Repository) Operation {
	return &operation{repo: repo}
}

var _ Operation = (*operation)(nil)

func (o *operation) Create(ctx context.Context, driveID ulid.ULID, rootNodeID uuid.UUID) (*Superblock, error) {
	sb := New(driveID, rootNodeID)
	if err := o.repo.Create(ctx, sb); err != nil {
		return nil, err
	}
	return sb, nil
}

func (o *operation) GetRootNodeID(ctx context.Context, driveID ulid.ULID) (uuid.UUID, error) {
	sb, err := o.repo.GetByDriveID(ctx, driveID)
	if err != nil {
		return uuid.Nil, err
	}
	return sb.RootNodeID(), nil
}

func (o *operation) DeleteByDriveID(ctx context.Context, driveID ulid.ULID) error {
	return o.repo.DeleteByDriveID(ctx, driveID)
}