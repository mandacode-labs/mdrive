package fs

import (
	"context"
	"encoding/json"

	"github.com/mandacode-labs/retrowin-go/internal/core/inode"
	"github.com/mandacode-labs/retrowin-go/internal/core/inode/content"
	"github.com/mandacode-labs/retrowin-go/internal/errors"
)

func (s *service) CreateFile(ctx context.Context, cmd *CreateFileCommand) (*inode.Inode, error) {
	if cmd.SystemID == "" {
		return nil, errors.BadRequest("system_id is required")
	}

	uid, gid, err := s.resolveIDs(ctx, cmd.SystemID, cmd.UID, cmd.GID)
	if err != nil {
		return nil, err
	}

	mode := cmd.Mode
	if mode == 0 {
		mode = inode.ModeRegular | inode.PermOwnerRW | inode.PermGroupRX | inode.PermOtherR
	}

	return s.inodeSvc.Create(ctx, &inode.CreateCommand{
		SystemID: cmd.SystemID,
		Mode:     mode,
		UID:      uid,
		GID:      gid,
		Size:     cmd.Size,
		Flags:    cmd.Flags,
		Content:  cmd.Content,
	})
}

func (s *service) CreateDirectory(ctx context.Context, cmd *CreateDirectoryCommand) (*inode.Inode, error) {
	if cmd.SystemID == "" {
		return nil, errors.BadRequest("system_id is required")
	}

	uid, gid, err := s.resolveIDs(ctx, cmd.SystemID, cmd.UID, cmd.GID)
	if err != nil {
		return nil, err
	}

	mode := cmd.Mode
	if mode == 0 {
		mode = inode.ModeDirectory | inode.PermOwnerRWX | inode.PermGroupRX | inode.PermOtherR
	}
	// Ensure directory bit is always set
	mode = mode | inode.ModeDirectory

	dirInode, err := s.inodeSvc.Create(ctx, &inode.CreateCommand{
		SystemID: cmd.SystemID,
		Mode:     mode,
		UID:      uid,
		GID:      gid,
		Flags:    cmd.Flags,
	})
	if err != nil {
		return nil, err
	}

	// Add "." entry to the new directory
	dotEntry := content.DirEntry{
		Name:     ".",
		InodeID:  dirInode.ID(),
		FileType: uint8(inode.ModeDirectory >> 12),
	}
	dirContent := content.DirContent{Entries: []content.DirEntry{dotEntry}}
	raw, err := json.Marshal(dirContent)
	if err != nil {
		return nil, err
	}
	if err := s.inodeSvc.Update(ctx, &inode.UpdateCommand{
		ID:      dirInode.ID(),
		Content: &raw,
	}); err != nil {
		return nil, err
	}

	return dirInode, nil
}

func (s *service) CreateSymlink(ctx context.Context, cmd *CreateSymlinkCommand) (*inode.Inode, error) {
	if cmd.SystemID == "" {
		return nil, errors.BadRequest("system_id is required")
	}
	if cmd.Target == "" {
		return nil, errors.BadRequest("target is required")
	}

	uid, gid, err := s.resolveIDs(ctx, cmd.SystemID, cmd.UID, cmd.GID)
	if err != nil {
		return nil, err
	}

	mode := cmd.Mode
	if mode == 0 {
		mode = inode.ModeSymlink | inode.PermOwnerRWX | inode.PermGroupRX | inode.PermOtherR
	}

	symContent := content.SymlinkContent{Target: cmd.Target}
	raw, err := json.Marshal(symContent)
	if err != nil {
		return nil, err
	}

	return s.inodeSvc.Create(ctx, &inode.CreateCommand{
		SystemID: cmd.SystemID,
		Mode:     mode,
		UID:      uid,
		GID:      gid,
		Flags:    cmd.Flags,
		Content:  raw,
	})
}

func (s *service) resolveIDs(ctx context.Context, systemID string, uid int, gid int) (int, int, error) {
	if uid >= 0 {
		return uid, gid, nil
	}

	resolvedUID, gids, err := s.userSvc.ResolveUIDAndGIDs(ctx, systemID)
	if err != nil {
		return 0, 0, err
	}

	if gid == 0 && len(gids) > 0 {
		gid = gids[0]
	}

	return resolvedUID, gid, nil
}
