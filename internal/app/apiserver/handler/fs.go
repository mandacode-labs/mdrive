package handler

import (
	"bytes"
	"context"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	apiv1 "github.com/mandacode-labs/mdrive/pkg/apiv1"
)

// --- FS (filesystem) handlers ---

func (h *Handler) Mkdir(ctx context.Context, req apiv1.OptMkdirReq, params apiv1.MkdirParams) error {
	r := req.Value
	_, err := h.vfs.Mkdir(ctx, h.userID(ctx), params.DriveID, r.Path)
	return err
}

func (h *Handler) Touch(ctx context.Context, req apiv1.OptTouchReq, params apiv1.TouchParams) error {
	r := req.Value
	_, err := h.vfs.Touch(ctx, h.userID(ctx), params.DriveID, r.Path)
	return err
}

func (h *Handler) Rm(ctx context.Context, req apiv1.OptRmReq, params apiv1.RmParams) error {
	r := req.Value
	rec := false
	if r.Recursive.Set {
		rec = bool(r.Recursive.Value)
	}
	return h.vfs.Rm(ctx, h.userID(ctx), params.DriveID, r.Paths, rec)
}

func (h *Handler) Mv(ctx context.Context, req apiv1.OptMvReq, params apiv1.MvParams) error {
	r := req.Value
	return h.vfs.Mv(ctx, h.userID(ctx), params.DriveID, r.Sources, params.DriveID, r.Destination)
}

func (h *Handler) Ls(ctx context.Context, params apiv1.LsParams) (*apiv1.DirContent, error) {
	path := params.Path
	if path == "" {
		path = "/"
	}
	dc, err := h.vfs.Ls(ctx, h.userID(ctx), params.DriveID, path)
	if err != nil {
		return nil, err
	}
	entries := make([]apiv1.DirEntry, len(dc.Entries))
	for i, e := range dc.Entries {
		entries[i] = apiv1.DirEntry{
			InodeID: apistr(e.InodeID.String()),
			Name:    apistr(e.Name),
			Type:    apistr(e.Type.String()),
		}
	}
	return &apiv1.DirContent{Entries: entries}, nil
}

func (h *Handler) Cat(ctx context.Context, params apiv1.CatParams) (apiv1.CatOK, error) {
	data, err := h.vfs.Cat(ctx, h.userID(ctx), params.DriveID, params.Path)
	if err != nil {
		return apiv1.CatOK{}, err
	}
	return apiv1.CatOK{Data: bytes.NewReader(data)}, nil
}

func (h *Handler) Write(ctx context.Context, req apiv1.OptWriteReq, params apiv1.WriteParams) error {
	r := req.Value
	return h.vfs.Write(ctx, h.userID(ctx), params.DriveID, r.Path, r.Content)
}

func (h *Handler) WriteLarge(ctx context.Context, req apiv1.OptWriteLargeReq, params apiv1.WriteLargeParams) (apiv1.WriteLargeRes, error) {
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
	return &apiv1.WriteLargeCreated{}, nil
}

func (h *Handler) Symlink(ctx context.Context, req apiv1.OptSymlinkReq, params apiv1.SymlinkParams) error {
	r := req.Value
	_, err := h.vfs.Symlink(ctx, h.userID(ctx), params.DriveID, r.Target, r.LinkPath)
	return err
}

func (h *Handler) Stat(ctx context.Context, params apiv1.StatParams) (*apiv1.StatOK, error) {
	n, err := h.vfs.Stat(ctx, h.userID(ctx), params.DriveID, params.Path)
	if err != nil {
		return nil, err
	}
	return &apiv1.StatOK{
		Type:     apistr(n.Type().String()),
		Size:     apiv1.OptInt64{Value: n.Size(), Set: true},
		Atime:    apiv1.OptDateTime{Value: n.ATime(), Set: true},
		Mtime:    apiv1.OptDateTime{Value: n.MTime(), Set: true},
		Ctime:    apiv1.OptDateTime{Value: n.CTime(), Set: true},
		Flags:    apistr(n.Flags().String()),
		Revision: apistr(n.Revision().String()),
	}, nil
}
