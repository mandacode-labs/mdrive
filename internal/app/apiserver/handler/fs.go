package handler

import (
	"bytes"
	"context"

	"github.com/mandacode-labs/mdrive/internal/app/apputils"
	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

// --- FS (filesystem) handlers ---

func (h *Handler) Mkdir(ctx context.Context, req api.OptMkdirReq, params api.MkdirParams) error {
	r := req.Value
	_, err := h.vfs.Mkdir(ctx, h.userID(ctx), params.DriveID, r.Path)
	return err
}

func (h *Handler) Touch(ctx context.Context, req api.OptTouchReq, params api.TouchParams) error {
	r := req.Value
	_, err := h.vfs.Touch(ctx, h.userID(ctx), params.DriveID, r.Path)
	return err
}

func (h *Handler) Rm(ctx context.Context, req api.OptRmReq, params api.RmParams) error {
	r := req.Value
	recursiveVal := false
	if r.Recursive.Set {
		recursiveVal = bool(r.Recursive.Value)
	}
	return h.vfs.Rm(ctx, h.userID(ctx), params.DriveID, r.Paths, recursiveVal)
}

func (h *Handler) Mv(ctx context.Context, req api.OptMvReq, params api.MvParams) error {
	r := req.Value
	return h.vfs.Mv(ctx, h.userID(ctx), params.DriveID, r.Sources, params.DriveID, r.Destination)
}

func (h *Handler) Ls(ctx context.Context, params api.LsParams) (*api.DirContent, error) {
	path := params.Path
	if path == "" {
		path = "/"
	}
	dc, err := h.vfs.Ls(ctx, h.userID(ctx), params.DriveID, path)
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
	data, err := h.vfs.Cat(ctx, h.userID(ctx), params.DriveID, params.Path)
	if err != nil {
		return api.CatOK{}, err
	}
	return api.CatOK{Data: bytes.NewReader(data)}, nil
}

func (h *Handler) Write(ctx context.Context, req api.OptWriteReq, params api.WriteParams) error {
	r := req.Value
	return h.vfs.Write(ctx, h.userID(ctx), params.DriveID, r.Path, r.Content)
}

func (h *Handler) WriteLarge(ctx context.Context, req api.OptWriteLargeReq, params api.WriteLargeParams) (api.WriteLargeRes, error) {
	r := req.Value
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
	if err := h.vfs.WriteLarge(ctx, h.userID(ctx), params.DriveID, r.Path, obj, r.Size); err != nil {
		return nil, err
	}
	return &api.WriteLargeCreated{}, nil
}

func (h *Handler) Symlink(ctx context.Context, req api.OptSymlinkReq, params api.SymlinkParams) error {
	r := req.Value
	_, err := h.vfs.Symlink(ctx, h.userID(ctx), params.DriveID, r.Target, r.LinkPath)
	return err
}

func (h *Handler) Stat(ctx context.Context, params api.StatParams) (*api.StatOK, error) {
	n, err := h.vfs.Stat(ctx, h.userID(ctx), params.DriveID, params.Path)
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
