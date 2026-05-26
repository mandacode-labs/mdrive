package fs

import (
	"context"
	"path"

	"github.com/mandacode-labs/retrowin-go/internal/core/dentry"
	"github.com/mandacode-labs/retrowin-go/internal/core/inode"
	"github.com/mandacode-labs/retrowin-go/internal/errors"
)

// Rm removes multiple paths, like Unix rm.
// For each path: resolve → lookup entry → unlinkat → delete inode.
func (s *service) Rm(ctx context.Context, cmd *RmCommand) (*RmResult, error) {
	if len(cmd.Paths) == 0 {
		return nil, errors.BadRequest("no paths provided")
	}

	result := &RmResult{}

	for _, p := range cmd.Paths {
		if err := s.rmOne(ctx, cmd.SystemID, p); err != nil {
			result.Errors = append(result.Errors, RmError{Path: p, Error: err})
			continue
		}
		result.Deleted = append(result.Deleted, p)
	}

	return result, nil
}

func (s *service) rmOne(ctx context.Context, systemID string, filePath string) error {
	dirPath := path.Dir(filePath)
	entryName := path.Base(filePath)

	parentDir, err := s.ResolvePath(ctx, systemID, dirPath)
	if err != nil {
		return err
	}

	entries, err := s.dentrySvc.ReadDir(ctx, parentDir.ID())
	if err != nil {
		return err
	}

	var targetEntry *dentry.DirEntry
	for i := range entries {
		if entries[i].Name == entryName {
			targetEntry = &entries[i]
			break
		}
	}
	if targetEntry == nil {
		return errors.NotFound("path not found: " + filePath)
	}

	if targetEntry.FileType == uint8(inode.ModeDirectory>>12) {
		targetInode, err := s.inodeSvc.GetByID(ctx, targetEntry.InodeID)
		if err != nil {
			return err
		}
		if !targetInode.IsEmptyDir() {
			return errors.BadRequest("directory not empty")
		}
	}

	if err := s.dentrySvc.Unlink(ctx, parentDir.ID(), entryName); err != nil {
		return err
	}

	return s.Delete(ctx, targetEntry.InodeID)
}
