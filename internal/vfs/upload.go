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
	// Write to registry FIRST — if this fails, no presigned URL is issued.
	if err := s.Reg.Put(ctx, meta, expiry); err != nil {
		return PresignInfo{}, fmt.Errorf("initiate upload: register: %w", err)
	}

	url, err := s.Store.GetPresignedUploadURL(ctx, st.Bucket(), key, ct, size, "", expiry)
	if err != nil {
		_ = s.Reg.Delete(ctx, uploadID)
		return PresignInfo{}, fmt.Errorf("initiate upload: presign: %w", err)
	}

	return PresignInfo{
		UploadID:  uploadID,
		Method:    "PUT",
		URL:       url,
		Headers:   map[string]string{},
		Key:       key,
		ExpiresAt: meta.ExpiresAt,
	}, nil
}

// CompleteUpload validates the upload token, verifies the S3 object exists,
// and creates the object node at the destination path.
func (s *Service) CompleteUpload(ctx context.Context, userID, driveID, uploadID string, contentLength int64, checksum *string) (*node.Node, error) {
	if err := s.checkAccess(ctx, userID, permission.PermissionEdit, driveID); err != nil {
		return nil, err
	}
	meta, err := s.Reg.Get(ctx, uploadID)
	if err != nil {
		return nil, fmt.Errorf("complete upload: get token: %w", err)
	}
	if meta.DriveID != driveID {
		return nil, ErrUploadMismatch
	}
	if meta.Size != nil && *meta.Size != contentLength {
		return nil, fmt.Errorf("complete upload: size mismatch: expected %d, got %d", *meta.Size, contentLength)
	}

	exists, err := s.Store.ObjectExists(ctx, meta.Bucket, meta.Key)
	if err != nil {
		return nil, fmt.Errorf("complete upload: check object: %w", err)
	}
	if !exists {
		return nil, ErrObjectNotUploaded
	}

	// Re-encrypt the body in place. The presigned URL targets a
	// plaintext key; once the client finishes the PUT, the
	// cryptostore reads the plaintext, encrypts, and writes the
	// ciphertext back to the same key. The ObjectContent node
	// then points at the ciphertext blob. When s.Reencryptor is
	// nil (e.g. legacy or dev), the body stays plaintext.
	if s.Reencryptor != nil {
		if err := s.Reencryptor.MigratePlaintext(ctx, driveID, meta.Bucket, meta.Key, meta.Key); err != nil {
			return nil, fmt.Errorf("complete upload: re-encrypt: %w", err)
		}
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
		if derr := s.Node.Delete(ctx, n.ID()); derr != nil {
			return nil, fmt.Errorf("complete upload: link: %w (cleanup: %v)", lerr, derr)
		}
		return nil, fmt.Errorf("complete upload: link: %w", lerr)
	}
	if err := s.Reg.Delete(ctx, uploadID); err != nil {
		return n, fmt.Errorf("complete upload: cleanup token: %w", err)
	}
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
	out, err := s.path.resolve(ctx, rootID, filePath)
	if err != nil {
		return PresignInfo{}, fmt.Errorf("presign download: %w", err)
	}
	n := out.Node
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
