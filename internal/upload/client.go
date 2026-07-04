package upload

import (
	"context"
	"time"

	"github.com/mandacode-labs/mdrive/internal/core/node"
)

// Client is the handler-facing upload surface: presign-upload,
// complete-upload, presign-download. The DeleteObject method
// stays off the Client interface — only gc.UploadExpirer uses
// it, and it holds the *Service directly.
type Client interface {
	InitiateUpload(ctx context.Context, userID, driveID, destPath string, contentType *string, contentLength *int64, expiry time.Duration) (PresignInfo, error)
	CompleteUpload(ctx context.Context, userID, driveID, uploadID string, contentLength int64, checksum *string) (*node.Node, error)
	PresignDownload(ctx context.Context, userID, driveID, path string, expiry time.Duration) (PresignInfo, error)
}
