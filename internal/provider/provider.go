package provider

import (
	"context"
	"time"
)

// ProviderType identifies a storage backend.
type ProviderType string

const (
	ProviderTypeS3    ProviderType = "s3"
	ProviderTypeMinio ProviderType = "minio"
)

// ProviderConfig is the server-side storage configuration.
// All fields except Type are required; secret is the
// plaintext credential (encryption belongs to the caller
// of the application, not the provider).
type ProviderConfig struct {
	Type         ProviderType
	Endpoint     string
	AccessKey    string
	SecretKey    string
	Bucket       string
	Region       string
	UsePathStyle bool
}

// UploadInfo is the result of a presigned PUT. Key is what
// the client must echo back to Complete.
type UploadInfo struct {
	URL string
	Key string
}

// ObjectMetadata is the result of HeadObject. Mirrors what
// the backend reports about an existing object.
type ObjectMetadata struct {
	Bucket string
	Size   int64
	ETag   string
	MTime  time.Time
}

// StorageProvider is the storage backend abstraction. One
// instance per app-level storage config (singleton per
// process); methods are called with the target key.
type StorageProvider interface {
	// Ping verifies the backend is reachable and credentials
	// are valid.
	Ping(ctx context.Context) error

	// PresignUpload returns a presigned PUT URL for the given
	// key. The returned Key must be passed back to Complete.
	PresignUpload(ctx context.Context, key, contentType string, expiry time.Duration) (UploadInfo, error)

	// PresignDownload returns a presigned GET URL for the key.
	PresignDownload(ctx context.Context, key string, expiry time.Duration) (string, error)

	// HeadObject returns backend-reported metadata for the
	// key. Returns ErrNotFound if missing.
	HeadObject(ctx context.Context, key string) (ObjectMetadata, error)
}