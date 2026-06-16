// Package storage defines the storage abstraction interface used by the
// application/vfs package. The interface is declared in the consumer
// (vfs) following Go convention; this file only re-exports a minimal
// interface for the storage implementations to satisfy.
//
// Concrete storage packages (e.g., internal/storage/s3) implement this
// interface implicitly.
package storage

import (
	"context"
	"io"
	"time"
)

// Storage is the consumer-declared interface for S3-like operations.
// It is intended to be consumed by the application/vfs service, which
// composes the storage backend with the node domain.
type Storage interface {
	// PutObject uploads an object to the given bucket.
	PutObject(ctx context.Context, bucket, key string, reader io.Reader, size int64) error

	// GetObject downloads an object as a byte slice.
	GetObject(ctx context.Context, bucket, key string) ([]byte, error)

	// DeleteObject removes an object.
	DeleteObject(ctx context.Context, bucket, key string) error

	// DeleteObjects removes multiple objects in a single request.
	DeleteObjects(ctx context.Context, bucket string, keys []string) error

	// ObjectExists checks whether the object exists.
	ObjectExists(ctx context.Context, bucket, key string) (bool, error)

	// GetObjectSize returns the size of the object in bytes.
	GetObjectSize(ctx context.Context, bucket, key string) (int64, error)

	// GetObjectChecksum returns the ETag (or hash) of the object.
	GetObjectChecksum(ctx context.Context, bucket, key string) (string, error)

	// GetPresignedUploadURL returns a presigned PUT URL for direct client upload.
	GetPresignedUploadURL(ctx context.Context, bucket, key, contentType string, size int64, checksum string, expiry time.Duration) (string, error)

	// GetPresignedDownloadURL returns a presigned GET URL.
	GetPresignedDownloadURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error)
}
