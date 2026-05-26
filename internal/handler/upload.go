package handler

import (
	"context"
	"net/url"
	"path"

	api "github.com/mandacode-labs/retrowin-go/pkg/api"

	"github.com/mandacode-labs/retrowin-go/internal/application/storage"
	"github.com/mandacode-labs/retrowin-go/internal/core/dentry"
	"github.com/mandacode-labs/retrowin-go/internal/core/inode"
	"github.com/mandacode-labs/retrowin-go/internal/errors"
)

// InitiateUpload implements POST /fs/{systemId}/upload/initiate.
func (h *Handler) InitiateUpload(ctx context.Context, req *api.InitiateUploadRequest, params api.InitiateUploadParams) (api.InitiateUploadRes, error) {
	if err := h.checkSystemAccess(ctx, params.SystemId); err != nil {
		return nil, h.domainError(err)
	}

	if req.Path == "" || req.Path[0] != '/' {
		return nil, errors.BadRequest("path must be absolute (start with /)")
	}
	if req.Size <= 0 {
		return nil, errors.BadRequest("size must be positive")
	}

	var contentType string
	if req.ContentType.Set {
		contentType = req.ContentType.Value
	}

	session, err := h.storageSvc.InitiateUpload(ctx, &storage.InitiateUploadCommand{
		SystemID:    params.SystemId,
		ContentType: contentType,
		Size:        req.Size,
	})
	if err != nil {
		return nil, h.domainError(err)
	}

	uploadURL, _ := url.Parse(session.UploadURL)

	return &api.UploadSessionResponse{
		UploadSession: api.UploadSession{
			ObjectId:  session.ObjectID,
			UploadUrl: *uploadURL,
			ExpiresAt: toTimestamp(session.ExpiresAt),
		},
	}, nil
}

// CompleteUpload implements POST /fs/{systemId}/upload/complete.
func (h *Handler) CompleteUpload(ctx context.Context, req *api.CompleteUploadRequest, params api.CompleteUploadParams) (api.CompleteUploadRes, error) {
	if err := h.checkSystemAccess(ctx, params.SystemId); err != nil {
		return nil, h.domainError(err)
	}

	// Validate and parse path
	if req.Path == "" || req.Path[0] != '/' {
		return nil, h.domainError(errors.BadRequest("path must be absolute (start with /)"))
	}

	dirPath := path.Dir(req.Path)
	fileName := path.Base(req.Path)

	mode := inode.ModeObject | inode.PermOwnerRW | inode.PermGroupRX | inode.PermOtherR
	if req.Mode.Set {
		mode = inode.ModeObject | int(req.Mode.Value)
	}

	result, err := h.storageSvc.CompleteUpload(ctx, &storage.CompleteUploadCommand{
		ObjectID: req.ObjectId,
		SystemID: params.SystemId,
		Mode:     mode,
	})
	if err != nil {
		return nil, h.domainError(err)
	}

	// Resolve parent directory
	parentDir, err := h.fsSvc.ResolvePath(ctx, params.SystemId, dirPath)
	if err != nil {
		return nil, h.domainError(err)
	}

	// Atomically replace or add the directory entry (reduces race window)
	entry := dentry.DirEntry{
		Name:     fileName,
		InodeID:  result.Inode.ID(),
		FileType: uint8(inode.ModeObject >> 12),
	}
	prevInodeID, err := h.dentrySvc.RenameAt(ctx, parentDir.ID(), entry)
	if err != nil {
		return nil, h.domainError(err)
	}

	// Clean up previous inode if replaced (best-effort)
	if prevInodeID != "" {
		_ = h.fsSvc.Delete(ctx, prevInodeID)
	}

	return &api.InodeResponse{
		Inode: *h.toInode(result.Inode),
	}, nil
}

// GetDownloadUrl implements GET /fs/{systemId}/download.
func (h *Handler) GetDownloadUrl(ctx context.Context, params api.GetDownloadUrlParams) (api.GetDownloadUrlRes, error) {
	if err := h.checkSystemAccess(ctx, params.SystemId); err != nil {
		return nil, h.domainError(err)
	}

	// First resolve the path to get inode ID
	in, err := h.fsSvc.ResolvePath(ctx, params.SystemId, params.Path)
	if err != nil {
		return nil, h.domainError(err)
	}

	downloadURL, expiresAt, err := h.storageSvc.GetDownloadURL(ctx, in.ID())
	if err != nil {
		return nil, h.domainError(err)
	}

	parsedURL, _ := url.Parse(downloadURL)

	return &api.DownloadURLResponse{
		DownloadUrl: api.DownloadURL{
			DownloadUrl: *parsedURL,
			ExpiresAt:   toTimestamp(expiresAt),
		},
	}, nil
}
