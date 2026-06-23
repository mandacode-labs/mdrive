package handler

import (
	"bytes"
	"context"

	"github.com/mandacode-labs/mdrive/internal/app/apputils"
	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

// --- FS (filesystem) handlers ---
//
// Permission checks live here in the handler. vfs is filesystem-only;
// each op here is preceded by a permission check on the drive
// (and on the resolved drive for read ops that may cross mounts).

func (h *Handler) Mkdir(ctx context.Context, req api.OptMkdirReq, params api.MkdirParams) error {
	r := req.Value
	if err := h.checkEdit(ctx, h.userID(ctx), params.DriveID); err != nil {
		return err
	}
	_, err := h.vfs.Mkdir(ctx, params.DriveID, r.Path)
	return err
}

func (h *Handler) Touch(ctx context.Context, req api.OptTouchReq, params api.TouchParams) error {
	r := req.Value
	if err := h.checkEdit(ctx, h.userID(ctx), params.DriveID); err != nil {
		return err
	}
	_, err := h.vfs.Touch(ctx, params.DriveID, r.Path)
	return err
}

func (h *Handler) Rm(ctx context.Context, req api.OptRmReq, params api.RmParams) error {
	r := req.Value
	if err := h.checkEdit(ctx, h.userID(ctx), params.DriveID); err != nil {
		return err
	}
	recursiveVal := false
	if r.Recursive.Set {
		recursiveVal = bool(r.Recursive.Value)
	}
	return h.vfs.Rm(ctx, params.DriveID, r.Paths, recursiveVal)
}

func (h *Handler) Mv(ctx context.Context, req api.OptMvReq, params api.MvParams) error {
	r := req.Value
	if err := h.checkEdit(ctx, h.userID(ctx), params.DriveID); err != nil {
		return err
	}
	return h.vfs.Mv(ctx, params.DriveID, r.Sources, params.DriveID, r.Destination)
}

func (h *Handler) Ls(ctx context.Context, params api.LsParams) (*api.DirContent, error) {
	path := params.Path
	if path == "" {
		path = "/"
	}
	// Resolve first so the permission check matches the drive the
	// path actually resolves to (a mount may have crossed).
	res, err := h.vfs.ResolveForPermission(ctx, params.DriveID, path)
	if err != nil {
		return nil, err
	}
	if err := h.checkViewAfterResolve(ctx, h.userID(ctx), res.DriveID); err != nil {
		return nil, err
	}
	// Re-resolve the remaining path (if any) within the source drive
	// so vfs.Ls sees a single-drive path it can walk cleanly.
	finalPath := res.Path
	if res.DriveID != params.DriveID {
		finalPath = "/" + res.Path
	}
	dc, err := h.vfs.Ls(ctx, res.DriveID, finalPath)
	if err != nil {
		return nil, err
	}
	entries := make([]api.DirEntry, len(dc.Entries))
	for i, e := range dc.Entries {
		entries[i] = api.DirEntry{
			InodeID: apputils.OptString(e.InodeID.String()),
			Name:    apputils.OptString(e.Name),
			Type:    apputils.OptString(e.Type.String()),
		}
	}
	return &api.DirContent{Entries: entries}, nil
}

func (h *Handler) Cat(ctx context.Context, params api.CatParams) (api.CatOK, error) {
	res, err := h.vfs.ResolveForPermission(ctx, params.DriveID, params.Path)
	if err != nil {
		return api.CatOK{}, err
	}
	if err := h.checkViewAfterResolve(ctx, h.userID(ctx), res.DriveID); err != nil {
		return api.CatOK{}, err
	}
	finalPath := res.Path
	if res.DriveID != params.DriveID {
		finalPath = "/" + res.Path
	}
	data, err := h.vfs.Cat(ctx, res.DriveID, finalPath)
	if err != nil {
		return api.CatOK{}, err
	}
	return api.CatOK{Data: bytes.NewReader(data)}, nil
}

func (h *Handler) Write(ctx context.Context, req api.OptWriteReq, params api.WriteParams) error {
	r := req.Value
	if err := h.checkEdit(ctx, h.userID(ctx), params.DriveID); err != nil {
		return err
	}
	return h.vfs.Write(ctx, params.DriveID, r.Path, r.Content)
}

func (h *Handler) WriteLarge(ctx context.Context, req api.OptWriteLargeReq, params api.WriteLargeParams) (api.WriteLargeRes, error) {
	r := req.Value
	if err := h.checkEdit(ctx, h.userID(ctx), params.DriveID); err != nil {
		return nil, err
	}
	ct := ""
	if r.Object.ContentType.Set {
		ct = r.Object.ContentType.Value
	}
	cs := ""
	if r.Object.Checksum.Set {
		cs = r.Object.Checksum.Value
	}
	obj := node.ObjectContent{
		Bucket:   r.Object.Bucket,
		Key:      r.Object.Key,
		Mime:     ct,
		Checksum: cs,
	}
	if err := h.vfs.WriteLarge(ctx, params.DriveID, r.Path, obj, r.Size); err != nil {
		return nil, err
	}
	return &api.WriteLargeCreated{}, nil
}

func (h *Handler) Symlink(ctx context.Context, req api.OptSymlinkReq, params api.SymlinkParams) error {
	r := req.Value
	if err := h.checkEdit(ctx, h.userID(ctx), params.DriveID); err != nil {
		return err
	}
	_, err := h.vfs.Symlink(ctx, params.DriveID, r.Target, r.LinkPath)
	return err
}

func (h *Handler) Stat(ctx context.Context, params api.StatParams) (*api.StatOK, error) {
	res, err := h.vfs.ResolveForPermission(ctx, params.DriveID, params.Path)
	if err != nil {
		return nil, err
	}
	if err := h.checkViewAfterResolve(ctx, h.userID(ctx), res.DriveID); err != nil {
		return nil, err
	}
	finalPath := res.Path
	if res.DriveID != params.DriveID {
		finalPath = "/" + res.Path
	}
	n, err := h.vfs.Stat(ctx, res.DriveID, finalPath)
	if err != nil {
		return nil, err
	}
	return &api.StatOK{
		Type:     apputils.OptString(n.Type().String()),
		Size:     api.OptInt64{Value: n.Size(), Set: true},
		Atime:    api.OptDateTime{Value: n.ATime(), Set: true},
		Mtime:    api.OptDateTime{Value: n.MTime(), Set: true},
		Ctime:    api.OptDateTime{Value: n.CTime(), Set: true},
		Flags:    apputils.OptString(n.Flags().String()),
		Revision: apputils.OptString(n.Revision().String()),
	}, nil
}
