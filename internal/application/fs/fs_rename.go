package fs

import (
	"context"
	"path"

	"github.com/mandacode-labs/retrowin-go/internal/core/dentry"
	"github.com/mandacode-labs/retrowin-go/internal/core/inode"
	"github.com/mandacode-labs/retrowin-go/internal/errors"
	"github.com/mandacode-labs/retrowin-go/internal/utils"
)

// Rename renames a single entry within the same directory.
func (s *service) Rename(ctx context.Context, cmd *RenameCommand) (*inode.Inode, error) {
	if cmd.NewName == "" {
		return nil, errors.BadRequest("new name is required")
	}
	if path.Base(cmd.NewName) != cmd.NewName {
		return nil, errors.BadRequest("new name must be a simple name, not a path")
	}

	sourceInode, err := s.ResolvePath(ctx, cmd.SystemID, cmd.Path)
	if err != nil {
		return nil, err
	}

	srcDirPath := path.Dir(cmd.Path)
	srcEntryName := path.Base(cmd.Path)

	sourceParentDir, err := s.ResolvePath(ctx, cmd.SystemID, srcDirPath)
	if err != nil {
		return nil, err
	}

	entries, err := s.dentrySvc.ReadDir(ctx, sourceParentDir.ID())
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.Name == cmd.NewName {
			return nil, errors.Conflict("target already exists")
		}
	}

	newEntry := dentry.DirEntry{
		Name:     cmd.NewName,
		InodeID:  sourceInode.ID(),
		FileType: utils.SafeIntToUint8(sourceInode.Mode() >> 12),
	}
	if err := s.dentrySvc.Link(ctx, sourceParentDir.ID(), newEntry); err != nil {
		return nil, err
	}

	if err := s.dentrySvc.Unlink(ctx, sourceParentDir.ID(), srcEntryName); err != nil {
		return nil, err
	}

	return s.Get(ctx, sourceInode.ID())
}
