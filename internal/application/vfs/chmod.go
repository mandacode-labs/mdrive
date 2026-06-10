package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/core/inode"
	"github.com/mandacode-labs/mdrive/internal/errors"
)

// ChmodPath changes permissions of a path.
func (s *service) ChmodPath(ctx context.Context, systemID string, filePath string, mode int) (*inode.Inode, error) {
	if mode < 0 || mode > 0o777 {
		return nil, errors.BadRequest("mode must be between 0 and 0o777")
	}

	in, err := s.ResolvePath(ctx, systemID, filePath)
	if err != nil {
		return nil, err
	}

	newMode := (in.Mode() & inode.ModeTypeMask) | mode
	if err := s.UpdateMode(ctx, &UpdateModeCommand{
		ID:   in.ID(),
		Mode: newMode,
	}); err != nil {
		return nil, err
	}

	return s.Get(ctx, in.ID())
}
