package handler

import (
	"bytes"
	"context"

	"github.com/mandacode-labs/mdrive/internal/app/apputils"
	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/permission"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

// --- FS (filesystem) handlers ---
//
// Permission checks live here in the handler. vfs is filesystem-only;
// each op here is preceded by a permission check on the drive
// (and on the resolved drive for read ops that may cross mounts).

func (h *Handler) Mkdir(ctx context.Context, req api.OptMkdirReq, params api.MkdirParams) (api.MkdirRes, error) {
	r := req.Value
	if err := h.requirePerm(ctx, permission.ActionEdit, params.DriveID); err != nil {
		return nil, err
	}
	if _, err := h.vfs.Mkdir(ctx, params.DriveID, r.Path); err != nil {
		return nil, err
	}
	return &api.MkdirOK{}, nil
}

func (h *Handler) Touch(ctx context.Context, req api.OptTouchReq, params api.TouchParams) (api.TouchRes, error) {
	r := req.Value
	if err := h.requirePerm(ctx, permission.ActionEdit, params.DriveID); err != nil {
		return nil, err
	}
	if _, err := h.vfs.Touch(ctx, params.DriveID, r.Path); err != nil {
		return nil, err
	}
	return &api.TouchOK{}, nil
}

func (h *Handler) Rm(ctx context.Context, req api.OptRmReq, params api.RmParams) (api.RmRes, error) {
	r := req.Value
	if err := h.requirePerm(ctx, permission.ActionEdit, params.DriveID); err != nil {
		return nil, err
	}
	recursiveVal := false
	if r.Recursive.Set {
		recursiveVal = bool(r.Recursive.Value)
	}
	if err := h.vfs.Rm(ctx, params.DriveID, r.Paths, recursiveVal); err != nil {
		return nil, err
	}
	return &api.RmNoContent{}, nil
}

func (h *Handler) Mv(ctx context.Context, req api.OptMvReq, params api.MvParams) (api.MvRes, error) {
	r := req.Value
	if err := h.requirePerm(ctx, permission.ActionEdit, params.DriveID); err != nil {
		return nil, err
	}
	if err := h.vfs.Mv(ctx, params.DriveID, r.Sources, params.DriveID, r.Destination); err != nil {
		return nil, err
	}
	return &api.MvOK{}, nil
}

func (h *Handler) Ls(ctx context.Context, params api.LsParams) (api.LsRes, error) {
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
	if err := h.requirePerm(ctx, permission.ActionView, res.DriveID); err != nil {
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

func (h *Handler) Cat(ctx context.Context, params api.CatParams) (api.CatRes, error) {
	res, err := h.vfs.ResolveForPermission(ctx, params.DriveID, params.Path)
	if err != nil {
		return nil, err
	}
	if err := h.requirePerm(ctx, permission.ActionView, res.DriveID); err != nil {
		return nil, err
	}
	finalPath := res.Path
	if res.DriveID != params.DriveID {
		finalPath = "/" + res.Path
	}
	data, err := h.vfs.Cat(ctx, res.DriveID, finalPath)
	if err != nil {
		return nil, err
	}
	return &api.CatOK{Data: bytes.NewReader(data)}, nil
}

func (h *Handler) Write(ctx context.Context, req api.OptWriteReq, params api.WriteParams) (api.WriteRes, error) {
	r := req.Value
	if err := h.requirePerm(ctx, permission.ActionEdit, params.DriveID); err != nil {
		return nil, err
	}
	if err := h.vfs.Write(ctx, params.DriveID, r.Path, r.Content); err != nil {
		return nil, err
	}
	return &api.WriteOK{}, nil
}

func (h *Handler) WriteLarge(ctx context.Context, req api.OptWriteLargeReq, params api.WriteLargeParams) (api.WriteLargeRes, error) {
	r := req.Value
	if err := h.requirePerm(ctx, permission.ActionEdit, params.DriveID); err != nil {
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
	return &api.WriteLargeOK{}, nil
}

func (h *Handler) Symlink(ctx context.Context, req api.OptSymlinkReq, params api.SymlinkParams) (api.SymlinkRes, error) {
	r := req.Value
	if err := h.requirePerm(ctx, permission.ActionEdit, params.DriveID); err != nil {
		return nil, err
	}
	if _, err := h.vfs.Symlink(ctx, params.DriveID, r.Target, r.LinkPath); err != nil {
		return nil, err
	}
	return &api.SymlinkOK{}, nil
}

func (h *Handler) Hardlink(ctx context.Context, req api.OptHardlinkReq, params api.HardlinkParams) (api.HardlinkRes, error) {
	r := req.Value
	if err := h.requirePerm(ctx, permission.ActionEdit, params.DriveID); err != nil {
		return nil, err
	}
	if _, err := h.vfs.Hardlink(ctx, params.DriveID, r.SrcPath, r.LinkPath); err != nil {
		return nil, err
	}
	return &api.HardlinkOK{}, nil
}

func (h *Handler) Mount(ctx context.Context, req api.OptMountReq, params api.MountParams) (api.MountRes, error) {
	r := req.Value
	if err := h.requirePerm(ctx, permission.ActionEdit, params.DriveID); err != nil {
		return nil, err
	}
	// View perm on the source drive: the mount makes the source's
	// root resolvable from this drive, so a non-viewable source
	// would leak the source's existence to anyone who can see
	// the mount point.
	if err := h.requirePerm(ctx, permission.ActionView, r.SourceDriveID); err != nil {
		return nil, err
	}
	if err := h.vfs.Mount(ctx, params.DriveID, r.MountPath, r.SourceDriveID); err != nil {
		return nil, err
	}
	return &api.MountOK{}, nil
}

func (h *Handler) Unmount(ctx context.Context, params api.UnmountParams) (api.UnmountRes, error) {
	if err := h.requirePerm(ctx, permission.ActionEdit, params.DriveID); err != nil {
		return nil, err
	}
	if err := h.vfs.Unmount(ctx, params.DriveID, params.MountPath); err != nil {
		return nil, err
	}
	return &api.UnmountNoContent{}, nil
}

func (h *Handler) Realpath(ctx context.Context, params api.RealpathParams) (api.RealpathRes, error) {
	if err := h.requirePerm(ctx, permission.ActionView, params.DriveID); err != nil {
		return nil, err
	}
	res, err := h.vfs.ResolveForPermission(ctx, params.DriveID, params.Path)
	if err != nil {
		return nil, err
	}
	return &api.RealpathOK{DriveID: res.DriveID, Path: res.Path}, nil
}

func (h *Handler) Stat(ctx context.Context, params api.StatParams) (api.StatRes, error) {
	res, err := h.vfs.ResolveForPermission(ctx, params.DriveID, params.Path)
	if err != nil {
		return nil, err
	}
	if err := h.requirePerm(ctx, permission.ActionView, res.DriveID); err != nil {
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
	return statToAPI(n), nil
}

// Lstat is the no-symlink-follow variant of Stat (POSIX lstat(2)).
// If the path resolves to a symlink, the returned metadata describes
// the symlink itself, not its target.
func (h *Handler) Lstat(ctx context.Context, params api.LstatParams) (api.LstatRes, error) {
	ref, err := h.vfs.ResolveForPermission(ctx, params.DriveID, params.Path)
	if err != nil {
		return nil, err
	}
	if err := h.requirePerm(ctx, permission.ActionView, ref.DriveID); err != nil {
		return nil, err
	}
	finalPath := ref.Path
	if ref.DriveID != params.DriveID {
		finalPath = "/" + ref.Path
	}
	res, err := h.vfs.Lstat(ctx, ref.DriveID, finalPath)
	if err != nil {
		return nil, err
	}
	return lstatToAPI(res.Node), nil
}

// Readlink returns the target path of a symbolic link (POSIX
// readlink(2)). The path must resolve to a symlink; otherwise
// ErrInvalidType is returned.
func (h *Handler) Readlink(ctx context.Context, params api.ReadlinkParams) (api.ReadlinkRes, error) {
	res, err := h.vfs.ResolveForPermission(ctx, params.DriveID, params.Path)
	if err != nil {
		return nil, err
	}
	if err := h.requirePerm(ctx, permission.ActionView, res.DriveID); err != nil {
		return nil, err
	}
	finalPath := res.Path
	if res.DriveID != params.DriveID {
		finalPath = "/" + res.Path
	}
	out, err := h.vfs.Lstat(ctx, res.DriveID, finalPath)
	if err != nil {
		return nil, err
	}
	target, err := out.Node.Readlink()
	if err != nil {
		return nil, err
	}
	return &api.ReadlinkOK{Target: target}, nil
}

func statToAPI(n *node.Node) *api.NodeStat {
	return &api.NodeStat{
		Type:     n.Type().String(),
		Size:     n.Size(),
		Mode:     n.Mode(),
		Nlink:    n.NLink(),
		Ino:      n.ID(),
		UID:      apputils.OptString(n.UID()),
		Gid:      apputils.OptString(n.GID()),
		Atime:    n.ATime(),
		Mtime:    n.MTime(),
		Ctime:    n.CTime(),
		Crtime:   n.CRTime(),
		Flags:    apputils.OptString(n.Flags().String()),
		Revision: apputils.OptString(n.Revision().String()),
	}
}

func lstatToAPI(n *node.Node) *api.NodeStat {
	return statToAPI(n)
}
