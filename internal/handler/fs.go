package handler

import (
	"context"
	"fmt"

	"github.com/mandacode-labs/mdrive/internal/core/inode"
	"github.com/mandacode-labs/mdrive/internal/utils"
	api "github.com/mandacode-labs/mdrive/pkg/api"
)

// GetRootDirectory implements GET /fs/{systemId}/root.
func (h *Handler) GetRootDirectory(ctx context.Context, params api.GetRootDirectoryParams) (api.GetRootDirectoryRes, error) {
	if err := h.checkSystemAccess(ctx, params.SystemId); err != nil {
		return nil, h.domainError(err)
	}

	rootInode, err := h.fsSvc.GetRootDirectory(ctx, params.SystemId)
	if err != nil {
		return nil, h.domainError(err)
	}

	inodeResp, err := h.toInode(rootInode)
	if err != nil {
		return nil, h.domainError(err)
	}
	return &api.InodeResponse{
		Inode: *inodeResp,
	}, nil
}

// StatPath implements GET /fs/{systemId}/stat.
func (h *Handler) StatPath(ctx context.Context, params api.StatPathParams) (api.StatPathRes, error) {
	if err := h.checkSystemAccess(ctx, params.SystemId); err != nil {
		return nil, h.domainError(err)
	}

	in, err := h.fsSvc.ResolvePath(ctx, params.SystemId, params.Path)
	if err != nil {
		return nil, h.domainError(err)
	}

	inodeResp, err := h.toInode(in)
	if err != nil {
		return nil, h.domainError(err)
	}
	return &api.InodeResponse{
		Inode: *inodeResp,
	}, nil
}

// Chmod implements PATCH /fs/{systemId}/chmod.
func (h *Handler) Chmod(ctx context.Context, req *api.ChmodRequest, params api.ChmodParams) (api.ChmodRes, error) {
	if err := h.checkSystemAccess(ctx, params.SystemId); err != nil {
		return nil, h.domainError(err)
	}

	updatedInode, err := h.fsSvc.ChmodPath(ctx, params.SystemId, req.Path, int(req.Mode))
	if err != nil {
		return nil, h.domainError(err)
	}

	inodeResp, err := h.toInode(updatedInode)
	if err != nil {
		return nil, h.domainError(err)
	}
	return &api.InodeResponse{
		Inode: *inodeResp,
	}, nil
}

func (h *Handler) toInode(in *inode.Inode) (*api.Inode, error) {
	mode, err := utils.SafeIntToInt32(in.Mode())
	if err != nil {
		return nil, fmt.Errorf("invalid mode: %w", err)
	}
	uid, err := utils.SafeIntToInt32(in.UID())
	if err != nil {
		return nil, fmt.Errorf("invalid uid: %w", err)
	}
	gid, err := utils.SafeIntToInt32(in.GID())
	if err != nil {
		return nil, fmt.Errorf("invalid gid: %w", err)
	}
	linkCount, err := utils.SafeIntToInt32(in.LinkCount())
	if err != nil {
		return nil, fmt.Errorf("invalid link count: %w", err)
	}
	flags, err := utils.SafeIntToInt32(in.Flags())
	if err != nil {
		return nil, fmt.Errorf("invalid flags: %w", err)
	}
	return &api.Inode{
		ID:        in.ID(),
		SystemId:  in.SystemID(),
		Mode:      mode,
		UID:       uid,
		Gid:       gid,
		Size:      in.Size(),
		LinkCount: linkCount,
		Flags:     flags,
		Atime:     toOptTimestamp(in.Atime()),
		Mtime:     toOptTimestamp(in.Mtime()),
		Ctime:     toOptTimestamp(in.Ctime()),
		CreatedAt: toOptTimestamp(in.CreatedAt()),
	}, nil
}
