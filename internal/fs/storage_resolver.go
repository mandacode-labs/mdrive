package fs

import (
	"context"
	"time"
)

// PresignInfo is the result of a presign operation.
type PresignInfo struct {
	Method    string
	URL       string
	Headers   map[string]string
	Key       string
	ExpiresAt time.Time
}

// Presigner is the storage backend abstraction bound to one
// bucket. Bucket is implicit (one Presigner per bucket);
// callers pass only the key.
type Presigner interface {
	Bucket() string
	PresignDownload(ctx context.Context, key string, expiry time.Duration) (string, error)
	PresignUpload(ctx context.Context, key, contentType string, expiry time.Duration) (PresignInfo, error)
	ObjectExists(ctx context.Context, key string) (bool, error)
	DeleteObject(ctx context.Context, key string) error
}

// StorageResolver picks the right Presigner for a drive.
// Per-drive Storage config takes priority; missing config
// falls back to the default (server-configured, e.g. IRSA).
type StorageResolver interface {
	Pick(ctx context.Context, driveID string) (Presigner, error)
}