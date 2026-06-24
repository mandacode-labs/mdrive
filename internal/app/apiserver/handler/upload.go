package handler

import (
	"context"
	"fmt"
	"net/url"

	"github.com/mandacode-labs/mdrive/internal/permission"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

// --- Presigned upload/download handlers ---

func (h *Handler) InitiateUpload(ctx context.Context, req api.OptPresignRequest, params api.InitiateUploadParams) (api.InitiateUploadRes, error) {
	if err := h.requirePerm(ctx, permission.PermissionEdit, params.DriveID); err != nil {
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
		return nil, err
	}
	u, err := url.Parse(info.URL)
	if err != nil {
		return nil, fmt.Errorf("parse presigned url: %w", err)
	}
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
	if err := h.requirePerm(ctx, permission.PermissionEdit, params.DriveID); err != nil {
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
		return nil, err
	}
	return &api.UploadCompleteResponse{
		InodeID: n.ID().String(),
		Size:    n.Size(),
		Mtime:   api.OptDateTime{Value: n.MTime(), Set: true},
		Atime:   api.OptDateTime{Value: n.ATime(), Set: true},
	}, nil
}

func (h *Handler) PresignDownload(ctx context.Context, params api.PresignDownloadParams) (api.PresignDownloadRes, error) {
	if err := h.requirePerm(ctx, permission.PermissionView, params.DriveID); err != nil {
		return nil, err
	}
	info, err := h.upload.PresignDownload(ctx, h.userID(ctx), params.DriveID, params.Path, h.presignTTL)
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(info.URL)
	if err != nil {
		return nil, fmt.Errorf("parse presigned url: %w", err)
	}
	return &api.DownloadResponse{
		Method:    info.Method,
		URL:       *u,
		ExpiresAt: info.ExpiresAt,
	}, nil
}
