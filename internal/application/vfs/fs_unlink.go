package vfs

import (
	"context"
	"path"

	"github.com/mandacode-labs/retrowin-go/internal/core/inode"
	"github.com/mandacode-labs/retrowin-go/internal/errors"
)

// UnlinkPath removes a path (file, symlink, or empty directory).
func (s *service) UnlinkPath(ctx context.Context, systemID string, filePath string) error {
	dirPath := path.Dir(filePath)
	entryName := path.Base(filePath)

	parentDir, err := s.ResolvePath(ctx, systemID, dirPath)
	if err != nil {
		return err
	}

	entries, err := s.inodeOps.ReadDir(ctx, parentDir.ID())
	if err != nil {
		return err
	}

	var targetEntry *inode.DirEntry
	for i := range entries {
		if entries[i].Name == entryName {
			targetEntry = &entries[i]
			break
		}
	}
	if targetEntry == nil {
		return errors.NotFound("path not found: " + filePath)
	}

	if err := s.inodeOps.Unlink(ctx, parentDir.ID(), entryName); err != nil {
		return err
	}

	if err := s.Delete(ctx, targetEntry.InodeID); err != nil && !errors.IsNotFound(err) {
		return err
	}

	return nil
}
