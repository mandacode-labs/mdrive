package handler

import (
	"context"

	"github.com/mandacode-labs/retrowin-go/internal/application/fs"
	api "github.com/mandacode-labs/retrowin-go/pkg/api"
)

// Ls implements GET /syscall/{systemId}/ls.
func (h *Handler) Ls(ctx context.Context, params api.LsParams) (api.LsRes, error) {
	if err := h.checkSystemAccess(ctx, params.SystemId); err != nil {
		return nil, h.domainError(err)
	}

	dirInode, err := h.fsSvc.ResolvePath(ctx, params.SystemId, params.Path)
	if err != nil {
		return nil, h.domainError(err)
	}

	entries, err := h.dentrySvc.ReadDir(ctx, dirInode.ID())
	if err != nil {
		return nil, h.domainError(err)
	}

	resp := &api.DirContentResponse{
		Entries: make([]api.DirEntry, len(entries)),
	}
	for i, e := range entries {
		resp.Entries[i] = api.DirEntry{
			Name:     e.Name,
			InodeId:  e.InodeID,
			FileType: int32(e.FileType),
		}
	}

	return resp, nil
}

// Mkdir implements POST /syscall/{systemId}/mkdir.
func (h *Handler) Mkdir(ctx context.Context, req *api.MkdirRequest, params api.MkdirParams) (api.MkdirRes, error) {
	if err := h.checkSystemAccess(ctx, params.SystemId); err != nil {
		return nil, h.domainError(err)
	}

	mode := 0
	if req.Mode.Set {
		mode = int(req.Mode.Value)
	}

	dirInode, err := h.fsSvc.Mkdir(ctx, params.SystemId, req.Path, mode)
	if err != nil {
		return nil, h.domainError(err)
	}

	return &api.InodeResponse{
		Inode: *h.toInode(dirInode),
	}, nil
}

// Ln implements POST /syscall/{systemId}/ln.
func (h *Handler) Ln(ctx context.Context, req *api.SymlinkRequest, params api.LnParams) (api.LnRes, error) {
	if err := h.checkSystemAccess(ctx, params.SystemId); err != nil {
		return nil, h.domainError(err)
	}

	symlinkInode, err := h.fsSvc.Ln(ctx, params.SystemId, req.LinkPath, req.Target)
	if err != nil {
		return nil, h.domainError(err)
	}

	return &api.InodeResponse{
		Inode: *h.toInode(symlinkInode),
	}, nil
}

// Unlink implements DELETE /syscall/{systemId}/unlink.
func (h *Handler) Unlink(ctx context.Context, params api.UnlinkParams) (api.UnlinkRes, error) {
	if err := h.checkSystemAccess(ctx, params.SystemId); err != nil {
		return nil, h.domainError(err)
	}

	if err := h.fsSvc.UnlinkPath(ctx, params.SystemId, params.Path); err != nil {
		return nil, h.domainError(err)
	}

	return &api.UnlinkNoContent{}, nil
}

// Rename implements POST /syscall/{systemId}/rename.
func (h *Handler) Rename(ctx context.Context, req *api.RenameRequest, params api.RenameParams) (api.RenameRes, error) {
	if err := h.checkSystemAccess(ctx, params.SystemId); err != nil {
		return nil, h.domainError(err)
	}

	renamedInode, err := h.fsSvc.Rename(ctx, &fs.RenameCommand{
		SystemID: params.SystemId,
		Path:     req.Path,
		NewName:  req.NewName,
	})
	if err != nil {
		return nil, h.domainError(err)
	}

	return &api.InodeResponse{
		Inode: *h.toInode(renamedInode),
	}, nil
}

// Mv implements POST /syscall/{systemId}/mv.
func (h *Handler) Mv(ctx context.Context, req *api.MvRequest, params api.MvParams) (api.MvRes, error) {
	if err := h.checkSystemAccess(ctx, params.SystemId); err != nil {
		return nil, h.domainError(err)
	}

	result, err := h.fsSvc.Mv(ctx, &fs.MvCommand{
		SystemID:    params.SystemId,
		Sources:     req.Sources,
		Destination: req.Destination,
	})
	if err != nil {
		return nil, h.domainError(err)
	}

	resp := &api.MvResult{
		Moved:  result.Moved,
		Errors: make([]api.MvResultErrorsItem, len(result.Errors)),
	}
	for i, e := range result.Errors {
		resp.Errors[i] = api.MvResultErrorsItem{
			Path:  e.Path,
			Error: e.Error.Error(),
		}
	}

	return resp, nil
}

// Rm implements POST /syscall/{systemId}/rm.
func (h *Handler) Rm(ctx context.Context, req *api.RmRequest, params api.RmParams) (api.RmRes, error) {
	if err := h.checkSystemAccess(ctx, params.SystemId); err != nil {
		return nil, h.domainError(err)
	}

	recursive := false
	if req.Recursive.Set {
		recursive = req.Recursive.Value
	}

	result, err := h.fsSvc.Rm(ctx, &fs.RmCommand{
		SystemID:  params.SystemId,
		Paths:     req.Paths,
		Recursive: recursive,
	})
	if err != nil {
		return nil, h.domainError(err)
	}

	resp := &api.RmResult{
		Deleted: result.Deleted,
		Errors:  make([]api.RmResultErrorsItem, len(result.Errors)),
	}
	for i, e := range result.Errors {
		resp.Errors[i] = api.RmResultErrorsItem{
			Path:  e.Path,
			Error: e.Error.Error(),
		}
	}

	return resp, nil
}
