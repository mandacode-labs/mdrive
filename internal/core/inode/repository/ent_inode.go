package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/mandacode-labs/retrowin-go/ent"
	entinode "github.com/mandacode-labs/retrowin-go/ent/inode"
	domain "github.com/mandacode-labs/retrowin-go/internal/core/inode"
)

// timeNow returns the current time. Extracted for testability.
var timeNow = time.Now

// EntRepository implements domain.InodeRepository using Ent.
type EntRepository struct {
	client *ent.Client
}

// NewRepository creates a new EntRepository.
func NewRepository(client *ent.Client) domain.InodeRepository {
	return &EntRepository{client: client}
}

func (r *EntRepository) Create(ctx context.Context, params *domain.CreateParams) (*domain.Inode, error) {
	now := timeNow()

	builder := r.client.Inode.Create().
		SetID(params.ID).
		SetSystemID(params.SystemID).
		SetMode(params.Mode).
		SetUID(params.UID).
		SetGid(params.GID).
		SetSize(params.Size).
		SetLinkCount(1).
		SetFlags(params.Flags).
		SetAtime(now).
		SetMtime(now).
		SetCtime(now)

	if params.Content != nil {
		builder.SetContent(params.Content)
	}

	entInode, err := builder.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create inode: %w", err)
	}

	return fromEnt(entInode), nil
}

func (r *EntRepository) GetByID(ctx context.Context, id string) (*domain.Inode, error) {
	entInode, err := r.client.Inode.Query().
		Where(entinode.ID(id)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get inode: %w", err)
	}
	return fromEnt(entInode), nil
}

func (r *EntRepository) Update(ctx context.Context, params *domain.UpdateParams) error {
	builder := r.client.Inode.UpdateOneID(params.ID)

	if params.Mode != nil {
		builder.SetMode(*params.Mode)
	}
	if params.UID != nil {
		builder.SetUID(*params.UID)
	}
	if params.GID != nil {
		builder.SetGid(*params.GID)
	}
	if params.Size != nil {
		builder.SetSize(*params.Size)
	}
	if params.Flags != nil {
		builder.SetFlags(*params.Flags)
	}
	if params.Content != nil {
		builder.SetContent(*params.Content)
	}
	if params.Atime != nil {
		builder.SetAtime(*params.Atime)
	}
	if params.Mtime != nil {
		builder.SetMtime(*params.Mtime)
	}
	if params.Ctime != nil {
		builder.SetCtime(*params.Ctime)
	}

	return builder.Exec(ctx)
}

func (r *EntRepository) Delete(ctx context.Context, id string) error {
	return r.client.Inode.DeleteOneID(id).Exec(ctx)
}

func (r *EntRepository) DeleteBySystemID(ctx context.Context, systemID string) error {
	_, err := r.client.Inode.Delete().Where(entinode.SystemIDEQ(systemID)).Exec(ctx)
	return err
}

func (r *EntRepository) Find(ctx context.Context, filter *domain.QueryFilter) ([]*domain.Inode, error) {
	query := r.client.Inode.Query()
	query = applyFilter(query, filter)

	entInodes, err := query.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find inodes: %w", err)
	}
	return fromEntSlice(entInodes), nil
}

func (r *EntRepository) FindOne(ctx context.Context, filter *domain.QueryFilter) (*domain.Inode, error) {
	query := r.client.Inode.Query()
	query = applyFilter(query, filter)

	entInode, err := query.Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find inode: %w", err)
	}
	return fromEnt(entInode), nil
}

func (r *EntRepository) UpdateLinkCount(ctx context.Context, id string, delta int) error {
	return r.client.Inode.UpdateOneID(id).
		AddLinkCount(delta).
		Exec(ctx)
}

func applyFilter(query *ent.InodeQuery, filter *domain.QueryFilter) *ent.InodeQuery {
	if filter == nil {
		return query
	}
	if filter.ID != nil {
		query = query.Where(entinode.ID(*filter.ID))
	}
	if len(filter.IDs) > 0 {
		query = query.Where(entinode.IDIn(filter.IDs...))
	}
	if filter.SystemID != nil {
		query = query.Where(entinode.SystemIDEQ(*filter.SystemID))
	}
	if filter.UID != nil {
		query = query.Where(entinode.UIDEQ(*filter.UID))
	}
	if filter.GID != nil {
		query = query.Where(entinode.GidEQ(*filter.GID))
	}
	return query
}

func fromEnt(e *ent.Inode) *domain.Inode {
	var content []byte
	if e.Content != nil {
		content = e.Content
	}

	return domain.NewInode(
		e.ID,
		e.SystemID,
		e.Mode,
		e.UID,
		e.Gid,
		e.Size,
		e.LinkCount,
		e.Flags,
		e.Atime,
		e.Mtime,
		e.Ctime,
		content,
		e.CreateTime,
		e.UpdateTime,
	)
}

func fromEntSlice(inodes []*ent.Inode) []*domain.Inode {
	result := make([]*domain.Inode, len(inodes))
	for i, e := range inodes {
		result[i] = fromEnt(e)
	}
	return result
}
