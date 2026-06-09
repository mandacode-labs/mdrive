package vfs

import (
	"context"

	"github.com/mandacode-labs/retrowin-go/internal/core/inode"
)

func (s *service) Get(ctx context.Context, id string) (*inode.Inode, error) {
	in, err := s.inodeOps.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := s.checkPermFromContext(ctx, in, AccessRead); err != nil {
		return nil, err
	}

	return in, nil
}

func (s *service) List(ctx context.Context, filter *ListFilter) ([]*inode.Inode, error) {
	f := inode.Filter{}
	if filter.SystemID != nil {
		f = inode.BySystemID(*filter.SystemID)
	}
	if filter.UID != nil {
		f.UID = filter.UID
	}
	return s.inodeOps.Find(ctx, f)
}
