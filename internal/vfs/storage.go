package vfs

import (
	"context"
	"io"
	"time"
)

// Storage is the consumer-declared interface for S3-like operations.
//
// Following Go convention, this interface is declared by the consumer
// (vfs), not by the implementation (internal/storage/s3). Concrete
// storage backends satisfy this interface implicitly.
type Storage interface {
	PutObject(ctx context.Context, bucket, key string, reader io.Reader, size int64) error
	GetObject(ctx context.Context, bucket, key string) ([]byte, error)
	DeleteObject(ctx context.Context, bucket, key string) error
	DeleteObjects(ctx context.Context, bucket string, keys []string) error
	ObjectExists(ctx context.Context, bucket, key string) (bool, error)
	GetObjectSize(ctx context.Context, bucket, key string) (int64, error)
	GetObjectChecksum(ctx context.Context, bucket, key string) (string, error)
	GetPresignedUploadURL(ctx context.Context, bucket, key, contentType string, size int64, checksum string, expiry time.Duration) (string, error)
	GetPresignedDownloadURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error)
}
