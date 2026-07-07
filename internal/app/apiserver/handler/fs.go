package handler

import (
	"bytes"
	"context"
	"log/slog"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
	"github.com/mandacode-labs/mdrive/internal/logx"
	"github.com/mandacode-labs/mdrive/internal/permission"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

// --- FS (filesystem) handlers ---
//
// Permission checks live here in the handler. fs is filesystem-
// only; each op here is preceded by a permission check on the
// drive (and on the resolved drive for read ops that may cross
// mounts).

func (h *Handler) Mkdir(ctx context.Context, req api.OptMkdirReq, params api.MkdirParams) (api.MkdirRes, error) {
	r := req.Value
	logx.Debug(ctx, "handler.fs.mkdir.enter",
		slog.String("drive_id", params.DriveID),
		slog.String("path", r.Path),
	)
	if err := h.requirePerm(ctx, permission.ActionEdit, params.DriveID); err != nil {
		logx.Debug(ctx, "handler.fs.mkdir.perm_denied",
			slog.String("drive_id", params.DriveID),
			slog.String("path", r.Path),
			slog.String("error_kind", errorx.KindOf(err).String()),
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	if _, err := h.fs.Create(ctx, params.DriveID, r.Path, fs.NodeKindDirectory); err != nil {
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
	if _, err := h.fs.Create(ctx, params.DriveID, r.Path, fs.NodeKindFile); err != nil {
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
	if err := h.fs.Remove(ctx, params.DriveID, r.Paths, fs.RemoveOpts{Recursive: recursiveVal}); err != nil {
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
	for _, src := range r.Sources {
		if err := h.fs.RenameAt(ctx, params.DriveID, src, params.DriveID, r.Destination); err != nil {
			logx.Debug(ctx, "handler.fs.mv.err",
				slog.String("drive_id", params.DriveID),
				slog.String("src", src),
				slog.String("err", err.Error()),
			)
			return nil, err
		}
	}
	logx.Debug(ctx, "handler.fs.mv.ok", slog.String("drive_id", params.DriveID))
	return &api.MvOK{}, nil
}

// resolveRead walks the path and gates ActionView on the
// resolved drive. Returns the drive id and the final path
// the caller should pass to fs.* (a leading "/" is prepended
// when the resolved drive differs from the requested one).
func (h *Handler) resolveRead(ctx context.Context, driveID, path string) (string, string, error) {
	driveULID, err := ulid.Parse(driveID)
	if err != nil {
		return "", "", errorx.New(errorx.KindInvalidArgument, "handler: invalid drive id")
	}
	dentry, err := h.fs.Walk(ctx, driveID, path)
	if err != nil {
		return "", "", err
	}
	resolvedDrive := dentry.DriveID
	if err := h.requirePerm(ctx, permission.ActionView, resolvedDrive.String()); err != nil {
		return "", "", err
	}
	finalPath := path
	if resolvedDrive != driveULID {
		finalPath = "/" + path
	}
	return resolvedDrive.String(), finalPath, nil
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
	resolvedDrive, finalPath, err := h.resolveRead(ctx, params.DriveID, path)
	if err != nil {
		return nil, err
	}
	entries, err := h.fs.Getdents(ctx, resolvedDrive, finalPath)
	if err != nil {
		logx.Debug(ctx, "handler.fs.ls.err",
			slog.String("drive_id", resolvedDrive),
			slog.String("path", finalPath),
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	out := make([]api.DirEntry, len(entries))
	for i, e := range entries {
		out[i] = api.DirEntry{
			InodeID: optString(e.InodeID.String()),
			Name:    optString(e.Name),
			Kind:    optString(e.Kind.String()),
		}
	}
	logx.Debug(ctx, "handler.fs.ls.ok",
		slog.String("drive_id", resolvedDrive),
		slog.String("path", finalPath),
		slog.Int("entry_count", len(out)),
	)
	return &api.DirContent{Entries: out}, nil
}

func (h *Handler) Cat(ctx context.Context, params api.CatParams) (api.CatRes, error) {
	logx.Debug(ctx, "handler.fs.cat.enter",
		slog.String("drive_id", params.DriveID),
		slog.String("path", params.Path),
	)
	resolvedDrive, finalPath, err := h.resolveRead(ctx, params.DriveID, params.Path)
	if err != nil {
		return nil, err
	}
	data, err := h.fs.Read(ctx, resolvedDrive, finalPath)
	if err != nil {
		logx.Debug(ctx, "handler.fs.cat.err",
			slog.String("drive_id", resolvedDrive),
			slog.String("path", finalPath),
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	logx.Debug(ctx, "handler.fs.cat.ok",
		slog.String("drive_id", resolvedDrive),
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
	ref := fs.ObjectRef{
		Bucket:   r.Object.Bucket,
		Key:      r.Object.Key,
		Mime:     ct,
		Checksum: cs,
	}
	logx.Debug(ctx, "handler.fs.write_large.enter",
		slog.String("drive_id", params.DriveID),
		slog.String("path", r.Path),
		slog.String("bucket", ref.Bucket),
		slog.Int64("size", r.Size),
	)
	if err := h.requirePerm(ctx, permission.ActionEdit, params.DriveID); err != nil {
		return nil, err
	}
	if _, err := h.fs.CreateObject(ctx, params.DriveID, r.Path, ref, r.Size); err != nil {
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
	if _, err := h.fs.SymlinkAt(ctx, params.DriveID, r.Target, r.LinkPath); err != nil {
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
	if _, err := h.fs.LinkAt(ctx, params.DriveID, r.SrcPath, r.LinkPath); err != nil {
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
	if err := h.requirePerm(ctx, permission.ActionView, r.SourceDriveID); err != nil {
		return nil, err
	}
	if err := h.fs.BindMount(ctx, params.DriveID, r.MountPath, r.SourceDriveID); err != nil {
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
	dentry, err := h.fs.Walk(ctx, params.DriveID, params.Path)
	if err != nil {
		logx.Debug(ctx, "handler.fs.realpath.err",
			slog.String("drive_id", params.DriveID),
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	logx.Debug(ctx, "handler.fs.realpath.ok",
		slog.String("drive_id", params.DriveID),
		slog.String("resolved_drive", dentry.DriveID.String()),
		slog.String("resolved_path", params.Path),
	)
	return &api.RealpathOK{DriveID: dentry.DriveID.String(), Path: params.Path}, nil
}

func (h *Handler) Stat(ctx context.Context, params api.StatParams) (api.StatRes, error) {
	logx.Debug(ctx, "handler.fs.stat.enter",
		slog.String("drive_id", params.DriveID),
		slog.String("path", params.Path),
	)
	resolvedDrive, finalPath, err := h.resolveRead(ctx, params.DriveID, params.Path)
	if err != nil {
		return nil, err
	}
	st, err := h.fs.Stat(ctx, resolvedDrive, finalPath, true)
	if err != nil {
		logx.Debug(ctx, "handler.fs.stat.err",
			slog.String("drive_id", resolvedDrive),
			slog.String("path", finalPath),
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	logx.Debug(ctx, "handler.fs.stat.ok", slog.String("drive_id", resolvedDrive))
	return statToAPI(st), nil
}

func (h *Handler) Lstat(ctx context.Context, params api.LstatParams) (api.LstatRes, error) {
	logx.Debug(ctx, "handler.fs.lstat.enter",
		slog.String("drive_id", params.DriveID),
		slog.String("path", params.Path),
	)
	resolvedDrive, finalPath, err := h.resolveRead(ctx, params.DriveID, params.Path)
	if err != nil {
		return nil, err
	}
	st, err := h.fs.Stat(ctx, resolvedDrive, finalPath, false)
	if err != nil {
		logx.Debug(ctx, "handler.fs.lstat.err",
			slog.String("drive_id", resolvedDrive),
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	logx.Debug(ctx, "handler.fs.lstat.ok", slog.String("drive_id", resolvedDrive))
	return statToAPI(st), nil
}

func (h *Handler) Readlink(ctx context.Context, params api.ReadlinkParams) (api.ReadlinkRes, error) {
	logx.Debug(ctx, "handler.fs.readlink.enter",
		slog.String("drive_id", params.DriveID),
		slog.String("path", params.Path),
	)
	resolvedDrive, finalPath, err := h.resolveRead(ctx, params.DriveID, params.Path)
	if err != nil {
		return nil, err
	}
	target, err := h.fs.ReadlinkAt(ctx, resolvedDrive, finalPath)
	if err != nil {
		logx.Debug(ctx, "handler.fs.readlink.err",
			slog.String("drive_id", resolvedDrive),
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	logx.Debug(ctx, "handler.fs.readlink.ok",
		slog.String("drive_id", resolvedDrive),
		slog.String("target", target),
	)
	return &api.ReadlinkOK{Target: target}, nil
}

func statToAPI(s fs.Stat) *api.NodeStat {
	return &api.NodeStat{
		Type:     s.Kind.String(),
		Size:     s.Size,
		Nlink:    s.NLink,
		Ino:      s.InodeID,
		Atime:    s.ATime,
		Mtime:    s.MTime,
		Ctime:    s.CTime,
		Crtime:   s.BTime,
		Flags:    optString(s.Flags.String()),
		Revision: optString(s.Revision.String()),
	}
}

// ensure filepath and uuid stay imported even if the toolchain
// trims them from a future pass; they document the dependency
// on parent-path handling and inode id round-tripping.
var (
	_ = filepath.Base
	_ = uuid.Nil
)
