package inode

import (
	"context"
	"time"
)

// InodeRepository defines the interface for inode data access.
type InodeRepository interface {
	Create(ctx context.Context, params *CreateParams) (*Inode, error)
	GetByID(ctx context.Context, id string) (*Inode, error)
	Update(ctx context.Context, params *UpdateParams) error
	Delete(ctx context.Context, id string) error
	DeleteBySystemID(ctx context.Context, systemID string) error
	Find(ctx context.Context, filter *QueryFilter) ([]*Inode, error)
	FindOne(ctx context.Context, filter *QueryFilter) (*Inode, error)
	UpdateLinkCount(ctx context.Context, id string, delta int) error
}

// CreateParams for creating a new inode (repository layer).
type CreateParams struct {
	ID       string
	SystemID string
	Mode     int
	UID      int
	GID      int
	Size     int64
	Flags    int
	Content  []byte
}

// UpdateParams for updating an inode (repository layer).
type UpdateParams struct {
	ID      string
	Mode    *int
	UID     *int
	GID     *int
	Size    *int64
	Flags   *int
	Content *[]byte
	Atime   *time.Time
	Mtime   *time.Time
	Ctime   *time.Time
}

// QueryFilter for querying inodes (repository layer).
type QueryFilter struct {
	ID       *string
	IDs      []string
	SystemID *string
	UID      *int
	GID      *int
	Mode     *int
}
