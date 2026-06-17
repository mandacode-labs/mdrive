package vfs

import (
	"context"
	"fmt"
	"path"
	"time"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/permission"
	"github.com/mandacode-labs/mdrive/internal/upload"
)

// PresignInfo is the result of initiating or presigning a download URL.
type PresignInfo struct {
	UploadID  string
	Method    string
	URL       string
	Headers   map[string]string
	Key       string
	ExpiresAt time.Time
}

// InitiateUpload creates a presigned PUT URL for uploading an object directly to S3.
// The returned uploadID must be passed to CompleteUpload after the client PUTs the bytes.
func (s *Service) InitiateUpload(ctx context.Context, userID, driveID, destPath string, contentType *string, contentLength *int64, expiry time.Duration) (PresignInfo, error) {
	if err := s.checkAccess(ctx, userID, permission.PermissionEdit, driveID); err != nil {
		return PresignInfo{}, err
	}
	st, err := s.Drive.GetStorage(ctx, driveID)
	if err != nil {
		return PresignInfo{}, err
	}
	uploadID := uuid.NewString()
	key := path.Join("drives", driveID, "uploads", uploadID)

	var ct string
	if contentType != nil {
		ct = *contentType
	}
	var size int64
	if contentLength != nil {
		size = *contentLength
	}

	url, err := s.Store.GetPresignedUploadURL(ctx, st.Bucket(), key, ct, size, "", expiry)
	if err != nil {
		return PresignInfo{}, fmt.Errorf("initiate upload: presign: %w", err)
	}

	meta := upload.PresignMeta{
		UploadID:    uploadID,
		DriveID:     driveID,
		UserID:      userID,
		Path:        destPath,
		Bucket:      st.Bucket(),
		Key:         key,
		ContentType: contentType,
		Size:        contentLength,
		ExpiresAt:   time.Now().Add(expiry),
	}
	if err := s.Reg.Put(ctx, meta, expiry); err != nil {
		return PresignInfo{}, fmt.Errorf("initiate upload: register: %w", err)
	}

	return PresignInfo{
		UploadID: uploadID,
		Method:   "PUT",
		URL:      url,
		Headers:  map[string]string{},
		Key:      key,
		ExpiresAt: meta.ExpiresAt,
	}, nil
}

// CompleteUpload validates the upload token and creates the object node at the destination path.
func (s *Service) CompleteUpload(ctx context.Context, userID, driveID, uploadID string, contentLength int64, checksum *string) (*node.Node, error) {
	if err := s.checkAccess(ctx, userID, permission.PermissionEdit, driveID); err != nil {
		return nil, err
	}
	meta, err := s.Reg.Get(ctx, uploadID)
	if err != nil {
		return nil, err
	}
	if meta.DriveID != driveID {
		return nil, ErrPermission
	}
	if meta.Size != nil && *meta.Size != contentLength {
		return nil, fmt.Errorf("complete upload: size mismatch: expected %d, got %d", *meta.Size, contentLength)
	}

	rootID, err := s.rootNodeID(ctx, driveID)
	if err != nil {
		return nil, err
	}
	parent, name, err := s.path.resolveParent(ctx, rootID, meta.Path)
	if err != nil {
		return nil, fmt.Errorf("complete upload: %w", err)
	}

	var ct string
	if meta.ContentType != nil {
		ct = *meta.ContentType
	}
	var cs string
	if checksum != nil {
		cs = *checksum
	}
	obj := node.ObjectContent{
		Bucket:   meta.Bucket,
		Key:      meta.Key,
		Mime:     ct,
		Checksum: cs,
	}
	n, err := s.Node.CreateObject(ctx, obj, contentLength)
	if err != nil {
		return nil, err
	}
	if lerr := s.Node.Link(ctx, parent, name, n); lerr != nil {
		_ = s.Node.Delete(ctx, n.ID())
		return nil, fmt.Errorf("complete upload: link: %w", lerr)
	}
	_ = s.Reg.Delete(ctx, uploadID)
	return n, nil
}

// PresignDownload returns a presigned GET URL for an existing object node.
func (s *Service) PresignDownload(ctx context.Context, userID, driveID, filePath string, expiry time.Duration) (PresignInfo, error) {
	if err := s.checkAccess(ctx, userID, permission.PermissionView, driveID); err != nil {
		return PresignInfo{}, err
	}
	rootID, err := s.rootNodeID(ctx, driveID)
	if err != nil {
		return PresignInfo{}, err
	}
	n, err := s.path.resolve(ctx, rootID, filePath)
	if err != nil {
		return PresignInfo{}, fmt.Errorf("presign download: %w", err)
	}
	if !n.IsObject() {
		return PresignInfo{}, fmt.Errorf("presign download: %s is not an object", n.Type())
	}
	oc, err := n.ReadObject()
	if err != nil {
		return PresignInfo{}, err
	}
	url, err := s.Store.GetPresignedDownloadURL(ctx, oc.Bucket, oc.Key, expiry)
	if err != nil {
		return PresignInfo{}, fmt.Errorf("presign download: %w", err)
	}
	return PresignInfo{
		Method:    "GET",
		URL:       url,
		ExpiresAt: time.Now().Add(expiry),
	}, nil
}
