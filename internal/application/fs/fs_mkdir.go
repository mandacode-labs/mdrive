package fs

import (
	"context"
	"path"

	"github.com/mandacode-labs/retrowin-go/internal/core/dentry"
	"github.com/mandacode-labs/retrowin-go/internal/core/inode"
	"github.com/mandacode-labs/retrowin-go/internal/errors"
)

// Mkdir creates a directory at the given path.
func (s *service) Mkdir(ctx context.Context, systemID string, filePath string, mode int) (*inode.Inode, error) {
	dirPath := path.Dir(filePath)
	dirName := path.Base(filePath)

	if dirPath == "/" && dirName == "/" {
		return nil, errors.BadRequest("cannot create root directory")
	}
	if dirName == "." || dirName == ".." {
		return nil, errors.BadRequest("invalid directory name: " + dirName)
	}

	parentDir, err := s.ResolvePath(ctx, systemID, dirPath)
	if err != nil {
		return nil, err
	}

	if mode == 0 {
		mode = inode.ModeDirectory | inode.PermOwnerRWX | inode.PermGroupRX | inode.PermOtherRX
	}

	dirInode, err := s.CreateDirectory(ctx, &CreateDirectoryCommand{
		SystemID: systemID,
		Mode:     mode,
	})
	if err != nil {
		return nil, err
	}

	// Link the new directory into its parent
	entry := dentry.DirEntry{
		Name:     dirName,
		InodeID:  dirInode.ID(),
		FileType: uint8(inode.ModeDirectory >> 12),
	}
	if err := s.dentrySvc.Link(ctx, parentDir.ID(), entry); err != nil {
		// Rollback: delete the orphaned inode
		_ = s.inodeSvc.Delete(ctx, dirInode.ID())
		return nil, err
	}

	// Add ".." entry to the new directory pointing to its parent
	dotdotEntry := dentry.DirEntry{
		Name:     "..",
		InodeID:  parentDir.ID(),
		FileType: uint8(inode.ModeDirectory >> 12),
	}
	if err := s.dentrySvc.Link(ctx, dirInode.ID(), dotdotEntry); err != nil {
		// Rollback: unlink from parent and delete the orphaned inode
		_ = s.dentrySvc.Unlink(ctx, parentDir.ID(), dirName)
		_ = s.inodeSvc.Delete(ctx, dirInode.ID())
		return nil, err
	}

	return dirInode, nil
}
