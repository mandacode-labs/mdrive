package handler

import (
	"bytes"
	"context"
	"log/slog"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/logx"
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
	logx.Debug(ctx, "handler.fs.mkdir.enter",
		slog.String("drive_id", params.DriveID),
		slog.String("path", r.Path),
	)
	if err := h.requirePerm(ctx, permission.ActionEdit, params.DriveID); err != nil {
		return nil, err
	}
	if _, err := h.fs.Mkdir(ctx, params.DriveID, r.Path); err != nil {
		logx.Debug(ctx, "handler.fs.mkdir.err",
			slog.String("drive_id", params.DriveID),
			slog.String("path", r.Path),
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	logx.Debug(ctx, "handler.fs.mkdir.ok",
		slog.String("drive_id", params.DriveID),
		slog.String("path", r.Path),
	)
	return &api.MkdirOK{}, nil
}

func (h *Handler) Touch(ctx context.Context, req api.OptTouchReq, params api.TouchParams) (api.TouchRes, error) {
	r := req.Value
	logx.Debug(ctx, "handler.fs.touch.enter",
		slog.String("drive_id", params.DriveID),
		slog.String("path", r.Path),
	)
	if err := h.requirePerm(ctx, permission.ActionEdit, params.DriveID); err != nil {
		return nil, err
	}
	if _, err := h.fs.Touch(ctx, params.DriveID, r.Path); err != nil {
		logx.Debug(ctx, "handler.fs.touch.err",
			slog.String("drive_id", params.DriveID),
			slog.String("path", r.Path),
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	logx.Debug(ctx, "handler.fs.touch.ok",
		slog.String("drive_id", params.DriveID),
		slog.String("path", r.Path),
	)
	return &api.TouchOK{}, nil
}

func (h *Handler) Rm(ctx context.Context, req api.OptRmReq, params api.RmParams) (api.RmRes, error) {
	r := req.Value
	recursiveVal := false
	if r.Recursive.Set {
		recursiveVal = bool(r.Recursive.Value)
	}
	logx.Debug(ctx, "handler.fs.rm.enter",
		slog.String("drive_id", params.DriveID),
		slog.Int("path_count", len(r.Paths)),
		slog.Bool("recursive", recursiveVal),
	)
	if err := h.requirePerm(ctx, permission.ActionEdit, params.DriveID); err != nil {
		return nil, err
	}
	if err := h.fs.Rm(ctx, params.DriveID, r.Paths, recursiveVal); err != nil {
		logx.Debug(ctx, "handler.fs.rm.err",
			slog.String("drive_id", params.DriveID),
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	logx.Debug(ctx, "handler.fs.rm.ok", slog.String("drive_id", params.DriveID))
	return &api.RmNoContent{}, nil
}

func (h *Handler) Mv(ctx context.Context, req api.OptMvReq, params api.MvParams) (api.MvRes, error) {
	r := req.Value
	logx.Debug(ctx, "handler.fs.mv.enter",
		slog.String("drive_id", params.DriveID),
		slog.Int("source_count", len(r.Sources)),
		slog.String("destination", r.Destination),
	)
	if err := h.requirePerm(ctx, permission.ActionEdit, params.DriveID); err != nil {
		return nil, err
	}
	if err := h.fs.Mv(ctx, params.DriveID, r.Sources, params.DriveID, r.Destination); err != nil {
		logx.Debug(ctx, "handler.fs.mv.err",
			slog.String("drive_id", params.DriveID),
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	logx.Debug(ctx, "handler.fs.mv.ok", slog.String("drive_id", params.DriveID))
	return &api.MvOK{}, nil
}

// resolveRead encapsulates the five-step preamble of every read
// handler that may cross a mount: ResolveForPermission, then
// requirePerm on the resolved drive, then compute a finalPath
// that includes a leading "/" when the resolved drive differs
// from the requested one. Returns the drive and final path to
// pass to vfs.* (and the absolute path the user requested) for
// any caller that needs it.
func (h *Handler) resolveRead(ctx context.Context, driveID, path string) (string, string, error) {
	res, err := h.fs.ResolveForPermission(ctx, driveID, path)
	if err != nil {
		return "", "", err
	}
	if err := h.requirePerm(ctx, permission.ActionView, res.DriveID); err != nil {
		return "", "", err
	}
	finalPath := res.Path
	if res.DriveID != driveID {
		finalPath = "/" + res.Path
	}
	return res.DriveID, finalPath, nil
}

func (h *Handler) Ls(ctx context.Context, params api.LsParams) (api.LsRes, error) {
	path := params.Path
	if path == "" {
		path = "/"
	}
	logx.Debug(ctx, "handler.fs.ls.enter",
		slog.String("drive_id", params.DriveID),
		slog.String("path", path),
	)
	driveID, finalPath, err := h.resolveRead(ctx, params.DriveID, path)
	if err != nil {
		return nil, err
	}
	dc, err := h.fs.Ls(ctx, driveID, finalPath)
	if err != nil {
		logx.Debug(ctx, "handler.fs.ls.err",
			slog.String("drive_id", driveID),
			slog.String("path", finalPath),
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	entries := make([]api.DirEntry, len(dc.Entries))
	for i, e := range dc.Entries {
		entries[i] = api.DirEntry{
			InodeID: optString(e.InodeID.String()),
			Name:    optString(e.Name),
			Kind:    optString(e.Kind.String()),
		}
	}
	logx.Debug(ctx, "handler.fs.ls.ok",
		slog.String("drive_id", driveID),
		slog.String("path", finalPath),
		slog.Int("entry_count", len(entries)),
	)
	return &api.DirContent{Entries: entries}, nil
}

func (h *Handler) Cat(ctx context.Context, params api.CatParams) (api.CatRes, error) {
	logx.Debug(ctx, "handler.fs.cat.enter",
		slog.String("drive_id", params.DriveID),
		slog.String("path", params.Path),
	)
	driveID, finalPath, err := h.resolveRead(ctx, params.DriveID, params.Path)
	if err != nil {
		return nil, err
	}
	data, err := h.fs.Cat(ctx, driveID, finalPath)
	if err != nil {
		logx.Debug(ctx, "handler.fs.cat.err",
			slog.String("drive_id", driveID),
			slog.String("path", finalPath),
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	logx.Debug(ctx, "handler.fs.cat.ok",
		slog.String("drive_id", driveID),
		slog.Int("bytes", len(data)),
	)
	return &api.CatOK{Data: bytes.NewReader(data)}, nil
}

func (h *Handler) Write(ctx context.Context, req api.OptWriteReq, params api.WriteParams) (api.WriteRes, error) {
	r := req.Value
	logx.Debug(ctx, "handler.fs.write.enter",
		slog.String("drive_id", params.DriveID),
		slog.String("path", r.Path),
	)
	if err := h.requirePerm(ctx, permission.ActionEdit, params.DriveID); err != nil {
		return nil, err
	}
	if err := h.fs.Write(ctx, params.DriveID, r.Path, r.Content); err != nil {
		logx.Debug(ctx, "handler.fs.write.err",
			slog.String("drive_id", params.DriveID),
			slog.String("path", r.Path),
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	logx.Debug(ctx, "handler.fs.write.ok",
		slog.String("drive_id", params.DriveID),
		slog.String("path", r.Path),
	)
	return &api.WriteOK{}, nil
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
	logx.Debug(ctx, "handler.fs.write_large.enter",
		slog.String("drive_id", params.DriveID),
		slog.String("path", r.Path),
		slog.String("bucket", obj.Bucket),
		slog.Int64("size", r.Size),
	)
	if err := h.requirePerm(ctx, permission.ActionEdit, params.DriveID); err != nil {
		return nil, err
	}
	if err := h.fs.WriteLarge(ctx, params.DriveID, r.Path, obj, r.Size); err != nil {
		logx.Debug(ctx, "handler.fs.write_large.err",
			slog.String("drive_id", params.DriveID),
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	logx.Debug(ctx, "handler.fs.write_large.ok", slog.String("drive_id", params.DriveID))
	return &api.WriteLargeOK{}, nil
}

func (h *Handler) Symlink(ctx context.Context, req api.OptSymlinkReq, params api.SymlinkParams) (api.SymlinkRes, error) {
	r := req.Value
	logx.Debug(ctx, "handler.fs.symlink.enter",
		slog.String("drive_id", params.DriveID),
		slog.String("target", r.Target),
		slog.String("link_path", r.LinkPath),
	)
	if err := h.requirePerm(ctx, permission.ActionEdit, params.DriveID); err != nil {
		return nil, err
	}
	if _, err := h.fs.Symlink(ctx, params.DriveID, r.Target, r.LinkPath); err != nil {
		logx.Debug(ctx, "handler.fs.symlink.err",
			slog.String("drive_id", params.DriveID),
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	logx.Debug(ctx, "handler.fs.symlink.ok", slog.String("drive_id", params.DriveID))
	return &api.SymlinkOK{}, nil
}

func (h *Handler) Hardlink(ctx context.Context, req api.OptHardlinkReq, params api.HardlinkParams) (api.HardlinkRes, error) {
	r := req.Value
	logx.Debug(ctx, "handler.fs.hardlink.enter",
		slog.String("drive_id", params.DriveID),
		slog.String("src_path", r.SrcPath),
		slog.String("link_path", r.LinkPath),
	)
	if err := h.requirePerm(ctx, permission.ActionEdit, params.DriveID); err != nil {
		return nil, err
	}
	if _, err := h.fs.Hardlink(ctx, params.DriveID, r.SrcPath, r.LinkPath); err != nil {
		logx.Debug(ctx, "handler.fs.hardlink.err",
			slog.String("drive_id", params.DriveID),
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	logx.Debug(ctx, "handler.fs.hardlink.ok", slog.String("drive_id", params.DriveID))
	return &api.HardlinkOK{}, nil
}

func (h *Handler) Mount(ctx context.Context, req api.OptMountReq, params api.MountParams) (api.MountRes, error) {
	r := req.Value
	logx.Debug(ctx, "handler.fs.mount.enter",
		slog.String("drive_id", params.DriveID),
		slog.String("mount_path", r.MountPath),
		slog.String("source_drive_id", r.SourceDriveID),
	)
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
	if err := h.fs.Mount(ctx, params.DriveID, r.MountPath, r.SourceDriveID); err != nil {
		logx.Debug(ctx, "handler.fs.mount.err",
			slog.String("drive_id", params.DriveID),
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	logx.Debug(ctx, "handler.fs.mount.ok", slog.String("drive_id", params.DriveID))
	return &api.MountOK{}, nil
}

func (h *Handler) Unmount(ctx context.Context, params api.UnmountParams) (api.UnmountRes, error) {
	logx.Debug(ctx, "handler.fs.unmount.enter",
		slog.String("drive_id", params.DriveID),
		slog.String("mount_path", params.MountPath),
	)
	if err := h.requirePerm(ctx, permission.ActionEdit, params.DriveID); err != nil {
		return nil, err
	}
	if err := h.fs.Unmount(ctx, params.DriveID, params.MountPath); err != nil {
		logx.Debug(ctx, "handler.fs.unmount.err",
			slog.String("drive_id", params.DriveID),
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	logx.Debug(ctx, "handler.fs.unmount.ok", slog.String("drive_id", params.DriveID))
	return &api.UnmountNoContent{}, nil
}

func (h *Handler) Realpath(ctx context.Context, params api.RealpathParams) (api.RealpathRes, error) {
	logx.Debug(ctx, "handler.fs.realpath.enter",
		slog.String("drive_id", params.DriveID),
		slog.String("path", params.Path),
	)
	if err := h.requirePerm(ctx, permission.ActionView, params.DriveID); err != nil {
		return nil, err
	}
	res, err := h.fs.ResolveForPermission(ctx, params.DriveID, params.Path)
	if err != nil {
		logx.Debug(ctx, "handler.fs.realpath.err",
			slog.String("drive_id", params.DriveID),
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	logx.Debug(ctx, "handler.fs.realpath.ok",
		slog.String("drive_id", params.DriveID),
		slog.String("resolved_drive", res.DriveID),
		slog.String("resolved_path", res.Path),
	)
	return &api.RealpathOK{DriveID: res.DriveID, Path: res.Path}, nil
}

func (h *Handler) Stat(ctx context.Context, params api.StatParams) (api.StatRes, error) {
	logx.Debug(ctx, "handler.fs.stat.enter",
		slog.String("drive_id", params.DriveID),
		slog.String("path", params.Path),
	)
	driveID, finalPath, err := h.resolveRead(ctx, params.DriveID, params.Path)
	if err != nil {
		return nil, err
	}
	n, err := h.fs.Stat(ctx, driveID, finalPath)
	if err != nil {
		logx.Debug(ctx, "handler.fs.stat.err",
			slog.String("drive_id", driveID),
			slog.String("path", finalPath),
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	logx.Debug(ctx, "handler.fs.stat.ok", slog.String("drive_id", driveID))
	return statToAPI(n), nil
}

// Lstat is the no-symlink-follow variant of Stat (POSIX lstat(2)).
// If the path resolves to a symlink, the returned metadata describes
// the symlink itself, not its target.
func (h *Handler) Lstat(ctx context.Context, params api.LstatParams) (api.LstatRes, error) {
	logx.Debug(ctx, "handler.fs.lstat.enter",
		slog.String("drive_id", params.DriveID),
		slog.String("path", params.Path),
	)
	driveID, finalPath, err := h.resolveRead(ctx, params.DriveID, params.Path)
	if err != nil {
		return nil, err
	}
	res, err := h.fs.Lstat(ctx, driveID, finalPath)
	if err != nil {
		logx.Debug(ctx, "handler.fs.lstat.err",
			slog.String("drive_id", driveID),
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	logx.Debug(ctx, "handler.fs.lstat.ok", slog.String("drive_id", driveID))
	return lstatToAPI(res.Node), nil
}

// Readlink returns the target path of a symbolic link (POSIX
// readlink(2)). The path must resolve to a symlink; otherwise
// ErrInvalidType is returned.
func (h *Handler) Readlink(ctx context.Context, params api.ReadlinkParams) (api.ReadlinkRes, error) {
	logx.Debug(ctx, "handler.fs.readlink.enter",
		slog.String("drive_id", params.DriveID),
		slog.String("path", params.Path),
	)
	driveID, finalPath, err := h.resolveRead(ctx, params.DriveID, params.Path)
	if err != nil {
		return nil, err
	}
	out, err := h.fs.Lstat(ctx, driveID, finalPath)
	if err != nil {
		logx.Debug(ctx, "handler.fs.readlink.err",
			slog.String("drive_id", driveID),
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	target, err := out.Node.Readlink()
	if err != nil {
		logx.Debug(ctx, "handler.fs.readlink.invalid_type",
			slog.String("drive_id", driveID),
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	logx.Debug(ctx, "handler.fs.readlink.ok",
		slog.String("drive_id", driveID),
		slog.String("target", target),
	)
	return &api.ReadlinkOK{Target: target}, nil
}

func statToAPI(n *node.Node) *api.NodeStat {
	return &api.NodeStat{
		Type:     n.Kind().String(),
		Size:     n.Size(),
		Nlink:    n.NLink(),
		Ino:      n.ID(),
		Atime:    n.ATime(),
		Mtime:    n.MTime(),
		Ctime:    n.CTime(),
		Crtime:   n.CRTime(),
		Flags:    optString(n.Flags().String()),
		Revision: optString(n.Revision().String()),
	}
}

func lstatToAPI(n *node.Node) api.LstatRes {
	return statToAPI(n)
}
