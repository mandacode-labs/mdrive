package handler

import (
	"context"
	"fmt"
	"net/url"
	"time"

	apiv1 "github.com/mandacode-labs/mdrive/pkg/apiv1"
)

// --- Presigned upload/download handlers ---

func (h *Handler) InitiateUpload(ctx context.Context, req apiv1.OptPresignRequest, params apiv1.InitiateUploadParams) (apiv1.InitiateUploadRes, error) {
	r := req.Value
	var ct *string
	if r.ContentType.Set {
		s := r.ContentType.Value
		ct = &s
	}
	var cl *int64
	if r.ContentLength.Set {
		v := r.ContentLength.Value
		cl = &v
	}
	info, err := h.vfs.InitiateUpload(ctx, h.userID(ctx), params.DriveID, r.Path, ct, cl, time.Hour)
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(info.URL)
	if err != nil {
		return nil, fmt.Errorf("parse presigned url: %w", err)
	}
	return &apiv1.PresignResponse{
		UploadId:  info.UploadID,
		Method:    info.Method,
		URL:       *u,
		Headers:   apiv1.OptPresignResponseHeaders{Value: info.Headers, Set: len(info.Headers) > 0},
		Key:       info.Key,
		ExpiresAt: info.ExpiresAt,
	}, nil
}

func (h *Handler) CompleteUpload(ctx context.Context, req apiv1.OptUploadCompleteRequest, params apiv1.CompleteUploadParams) (apiv1.CompleteUploadRes, error) {
	r := req.Value
	var cs *string
	if r.Checksum.Set {
		s := r.Checksum.Value
		cs = &s
	}
	n, err := h.vfs.CompleteUpload(ctx, h.userID(ctx), params.DriveID, params.UploadId, r.ContentLength, cs)
	if err != nil {
		return nil, err
	}
	return &apiv1.UploadCompleteResponse{
		InodeID: n.ID().String(),
		Size:    n.Size(),
		Mtime:   apiv1.OptDateTime{Value: n.MTime(), Set: true},
		Atime:   apiv1.OptDateTime{Value: n.ATime(), Set: true},
	}, nil
}

func (h *Handler) PresignDownload(ctx context.Context, params apiv1.PresignDownloadParams) (apiv1.PresignDownloadRes, error) {
	info, err := h.vfs.PresignDownload(ctx, h.userID(ctx), params.DriveID, params.Path, time.Hour)
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(info.URL)
	if err != nil {
		return nil, fmt.Errorf("parse presigned url: %w", err)
	}
	return &apiv1.DownloadResponse{
		Method:    info.Method,
		URL:       *u,
		ExpiresAt: info.ExpiresAt,
	}, nil
}
