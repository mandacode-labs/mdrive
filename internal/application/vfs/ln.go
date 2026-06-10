package vfs

import (
	"context"
	"path"

	"github.com/mandacode-labs/mdrive/internal/core/inode"
	"github.com/mandacode-labs/mdrive/internal/errors"
)

// Ln creates a symbolic link at linkPath pointing to target.
func (s *service) Ln(ctx context.Context, systemID string, linkPath string, target string) (*inode.Inode, error) {
	if target == "" {
		return nil, errors.BadRequest("target path is required")
	}
	if len(target) > 4096 {
		return nil, errors.BadRequest("target path too long")
	}

	linkDir := path.Dir(linkPath)
	linkName := path.Base(linkPath)

	parentDir, err := s.ResolvePath(ctx, systemID, linkDir)
	if err != nil {
		return nil, err
	}

	symlinkInode, err := s.CreateSymlink(ctx, &CreateSymlinkCommand{
		SystemID: systemID,
		Target:   target,
		Mode:     inode.ModeSymlink | 0777,
	})
	if err != nil {
		return nil, err
	}

	entry := inode.DirEntry{
		Name:     linkName,
		InodeID:  symlinkInode.ID(),
		FileType: uint8(inode.ModeSymlink >> 12),
	}
	if err := s.inodeOps.Link(ctx, parentDir.ID(), entry); err != nil {
		return nil, err
	}

	return symlinkInode, nil
}
