package fs

import (
	"context"
	"path"
	"strings"

	"github.com/mandacode-labs/retrowin-go/internal/core/dentry"
	"github.com/mandacode-labs/retrowin-go/internal/errors"
)

// Mv moves multiple sources to a destination, like Unix mv.
// Uses renameat (link + unlink) per source.
func (s *service) Mv(ctx context.Context, cmd *MvCommand) (*MvResult, error) {
	if len(cmd.Sources) == 0 {
		return nil, errors.BadRequest("no sources provided")
	}
	if cmd.Destination == "" {
		return nil, errors.BadRequest("destination is required")
	}

	result := &MvResult{}

	for _, src := range cmd.Sources {
		if err := s.mvOne(ctx, cmd.SystemID, src, cmd.Destination); err != nil {
			result.Errors = append(result.Errors, MvError{Path: src, Error: err})
			continue
		}
		result.Moved = append(result.Moved, src)
	}

	return result, nil
}

func (s *service) mvOne(ctx context.Context, systemID string, srcPath string, destPath string) error {
	if srcPath == destPath {
		return errors.BadRequest("source and destination are the same")
	}

	sourceInode, err := s.ResolvePath(ctx, systemID, srcPath)
	if err != nil {
		return err
	}

	srcDirPath := path.Dir(srcPath)
	srcEntryName := path.Base(srcPath)

	sourceParentDir, err := s.ResolvePath(ctx, systemID, srcDirPath)
	if err != nil {
		return err
	}

	var destDirPath, newEntryName string

	destInode, err := s.ResolvePath(ctx, systemID, destPath)
	if err == nil {
		if destInode.IsDir() {
			destDirPath = destPath
			newEntryName = srcEntryName
		} else {
			return errors.Conflict("target already exists")
		}
	} else {
		destDirPath = path.Dir(destPath)
		newEntryName = path.Base(destPath)
		if destDirPath == "." {
			destDirPath = "/"
		}
	}

	normalizedSource := path.Clean(srcPath)
	normalizedDest := path.Clean(destDirPath + "/" + newEntryName)
	if normalizedSource == normalizedDest {
		return errors.BadRequest("source and destination are the same")
	}

	if sourceInode.IsDir() {
		if strings.HasPrefix(normalizedDest, normalizedSource+"/") {
			return errors.BadRequest("cannot move directory into itself")
		}
	}

	destParentDir, err := s.ResolvePath(ctx, systemID, destDirPath)
	if err != nil {
		return err
	}

	destEntries, err := s.dentrySvc.ReadDir(ctx, destParentDir.ID())
	if err != nil {
		return err
	}
	for _, e := range destEntries {
		if e.Name == newEntryName {
			return errors.Conflict("target already exists")
		}
	}

	newEntry := dentry.DirEntry{
		Name:     newEntryName,
		InodeID:  sourceInode.ID(),
		FileType: uint8(sourceInode.Mode() >> 12),
	}
	if err := s.dentrySvc.Link(ctx, destParentDir.ID(), newEntry); err != nil {
		return err
	}

	return s.dentrySvc.Unlink(ctx, sourceParentDir.ID(), srcEntryName)
}
