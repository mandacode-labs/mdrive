package handler

import (
	"context"
	"log/slog"
	"net/url"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/logx"
	"github.com/mandacode-labs/mdrive/internal/permission"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

func (h *Handler) InitiateUpload(ctx context.Context, req api.OptPresignRequest, params api.InitiateUploadParams) (api.InitiateUploadRes, error) {
	logx.Debug(ctx, "handler.upload.initiate.enter",
		slog.String("drive_id", params.DriveID),
		slog.String("path", req.Value.Path),
	)
	if err := h.requirePerm(ctx, permission.ActionEdit, params.DriveID); err != nil {
		logx.Debug(ctx, "handler.upload.initiate.perm_denied",
			slog.String("drive_id", params.DriveID),
			slog.String("path", req.Value.Path),
			slog.String("error_kind", errorx.KindOf(err).String()),
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	r := req.Value
	var contentType *string
	if r.ContentType.Set {
		s := r.ContentType.Value
		contentType = &s
	}
	var contentLength *int64
	if r.ContentLength.Set {
		v := r.ContentLength.Value
		contentLength = &v
	}
	info, err := h.upload.InitiateUpload(ctx, h.userID(ctx), params.DriveID, r.Path, contentType, contentLength, h.presignTTL)
	if err != nil {
		logx.Debug(ctx, "handler.upload.initiate.service_err",
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	u, err := url.Parse(info.URL)
	if err != nil {
		return nil, errorx.Wrap(err, "upload: parse presigned url")
	}
	logx.Debug(ctx, "handler.upload.initiate.ok",
		slog.String("upload_id", info.UploadID),
	)
	return &api.PresignResponse{
		UploadId:  info.UploadID,
		Method:    info.Method,
		URL:       *u,
		Headers:   api.OptPresignResponseHeaders{Value: info.Headers, Set: len(info.Headers) > 0},
		Key:       info.Key,
		ExpiresAt: info.ExpiresAt,
	}, nil
}

func (h *Handler) CompleteUpload(ctx context.Context, req api.OptUploadCompleteRequest, params api.CompleteUploadParams) (api.CompleteUploadRes, error) {
	logx.Debug(ctx, "handler.upload.complete.enter",
		slog.String("drive_id", params.DriveID),
		slog.String("upload_id", params.UploadId),
	)
	if err := h.requirePerm(ctx, permission.ActionEdit, params.DriveID); err != nil {
		logx.Debug(ctx, "handler.upload.complete.perm_denied",
			slog.String("drive_id", params.DriveID),
			slog.String("upload_id", params.UploadId),
			slog.String("error_kind", errorx.KindOf(err).String()),
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	r := req.Value
	var cs *string
	if r.Checksum.Set {
		s := r.Checksum.Value
		cs = &s
	}
	n, err := h.upload.CompleteUpload(ctx, h.userID(ctx), params.DriveID, params.UploadId, r.ContentLength, cs)
	if err != nil {
		logx.Debug(ctx, "handler.upload.complete.service_err",
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	logx.Debug(ctx, "handler.upload.complete.ok",
		slog.String("inode_id", n.ID().String()),
		slog.Int64("size", n.Size()),
	)
	return &api.UploadCompleteResponse{
		InodeID: n.ID().String(),
		Size:    n.Size(),
		Mtime:   api.OptDateTime{Value: n.MTime(), Set: true},
		Atime:   api.OptDateTime{Value: n.ATime(), Set: true},
	}, nil
}

func (h *Handler) PresignDownload(ctx context.Context, params api.PresignDownloadParams) (api.PresignDownloadRes, error) {
	logx.Debug(ctx, "handler.upload.presign_download.enter",
		slog.String("drive_id", params.DriveID),
		slog.String("path", params.Path),
	)
	if err := h.requirePerm(ctx, permission.ActionView, params.DriveID); err != nil {
		logx.Debug(ctx, "handler.upload.presign_download.perm_denied",
			slog.String("drive_id", params.DriveID),
			slog.String("path", params.Path),
			slog.String("error_kind", errorx.KindOf(err).String()),
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	info, err := h.upload.PresignDownload(ctx, h.userID(ctx), params.DriveID, params.Path, h.presignTTL)
	if err != nil {
		logx.Debug(ctx, "handler.upload.presign_download.service_err",
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	u, err := url.Parse(info.URL)
	if err != nil {
		return nil, errorx.Wrap(err, "upload: parse presigned url")
	}
	logx.Debug(ctx, "handler.upload.presign_download.ok")
	return &api.DownloadResponse{
		Method:    info.Method,
		URL:       *u,
		ExpiresAt: info.ExpiresAt,
	}, nil
}
