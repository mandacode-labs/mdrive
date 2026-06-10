package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/core/inode"
)

func (s *service) UpdateContent(ctx context.Context, cmd *UpdateContentCommand) (*inode.Inode, error) {
	current, err := s.inodeOps.GetByID(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}

	if err := s.checkPermFromContext(ctx, current, AccessWrite); err != nil {
		return nil, err
	}

	size := int64(len(cmd.Content))
	if err := s.inodeOps.Update(ctx, &inode.UpdateCommand{
		ID:   cmd.ID,
		Size: &size,
	}); err != nil {
		return nil, err
	}

	return current, nil
}

func (s *service) UpdateMode(ctx context.Context, cmd *UpdateModeCommand) error {
	current, err := s.inodeOps.GetByID(ctx, cmd.ID)
	if err != nil {
		return err
	}

	if err := s.checkPermFromContext(ctx, current, AccessWrite); err != nil {
		return err
	}

	return s.inodeOps.Update(ctx, &inode.UpdateCommand{
		ID:   cmd.ID,
		Mode: &cmd.Mode,
	})
}
