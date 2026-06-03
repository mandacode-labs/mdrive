package object

import (
	"context"
	"io"
	"time"
)

// Storage defines the interface for file storage backends.
type Storage interface {
	// DefaultBucket returns the default bucket name from config.
	DefaultBucket() string

	// KeyPrefix returns the configured prefix for storage keys.
	KeyPrefix() string

	// PutObject streams data directly to storage. Returns the uploaded size.
	PutObject(ctx context.Context, bucket string, key string, reader io.Reader, size int64) error

	// GetPresignedDownloadURL generates a presigned URL for direct client download.
	GetPresignedDownloadURL(ctx context.Context, bucket string, key string, expiry time.Duration) (string, error)

	// GetPresignedUploadURL generates a presigned URL for direct client upload.
	// If checksum is provided, it is included in the presigned URL signature so S3 validates it on upload.
	GetPresignedUploadURL(ctx context.Context, bucket string, key string, contentType string, size int64, checksum string, expiry time.Duration) (string, error)

	DeleteObject(ctx context.Context, bucket string, key string) error
	ObjectExists(ctx context.Context, bucket string, key string) (bool, error)
	GetObjectSize(ctx context.Context, bucket string, key string) (int64, error)
	// GetObjectChecksum returns the content checksum (e.g., S3 ETag) for verification.
	GetObjectChecksum(ctx context.Context, bucket string, key string) (string, error)
}

const (
	DefaultDownloadExpiry = 1 * time.Hour
	DefaultUploadExpiry   = 1 * time.Hour
	MinExpiry             = 15 * time.Minute
	MaxExpiry             = 6 * time.Hour
)

// ExpiryForSize calculates a presigned URL expiry based on file size.
// Larger files get more time for upload/download.
func ExpiryForSize(size int64) time.Duration {
	const mb = 1 << 20
	switch {
	case size <= 10*mb: // <= 10MB
		return 15 * time.Minute
	case size <= 100*mb: // <= 100MB
		return 1 * time.Hour
	case size <= 1024*mb: // <= 1GB
		return 3 * time.Hour
	default: // > 1GB
		return 6 * time.Hour
	}
}
