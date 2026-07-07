package fs

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
)

// Storage is the per-superblock storage backend config. vfs
// owns this domain for presign/verify operations. drive has
// its own Storage struct for management (different key, same
// ent table).
//
// Storage does not carry the superblock_id itself — that
// was just the lookup key. The repo's GetBySuperblock does
// the superblock → drive join and returns this struct.
type Storage struct {
	driveID      ulid.ULID
	provider     string
	bucket       string
	endpoint     *string
	region       string
	accessKey    string
	secretKey    string
	usePathStyle bool
}

func (s *Storage) DriveID() ulid.ULID { return s.driveID }
func (s *Storage) Provider() string   { return s.provider }
func (s *Storage) Bucket() string     { return s.bucket }
func (s *Storage) Endpoint() *string  { return s.endpoint }
func (s *Storage) Region() string    { return s.region }
func (s *Storage) AccessKey() string  { return s.accessKey }
func (s *Storage) SecretKey() string  { return s.secretKey }
func (s *Storage) UsePathStyle() bool { return s.usePathStyle }

// NewStorage constructs a fresh Storage bound to a drive.
func NewStorage(
	driveID ulid.ULID,
	provider, bucket, region string,
	endpoint *string,
	accessKey, secretKey string,
	usePathStyle bool,
) *Storage {
	return &Storage{
		driveID:      driveID,
		provider:     provider,
		bucket:       bucket,
		endpoint:     endpoint,
		region:       region,
		accessKey:    accessKey,
		secretKey:    secretKey,
		usePathStyle: usePathStyle,
	}
}

// UploadInfo is returned by Service.Upload. The client uses
// the URL to PUT and the Key to call Service.Complete.
type UploadInfo struct {
	URL string
	Key string
}

// ObjectMetadata is returned by Service.Verify / vfs.Verify.
// It's what the backend (S3) reports about an existing
// object — not what the client claims.
type ObjectMetadata struct {
	Bucket string
	Size   int64
	ETag   string
	MTime  time.Time
}

// StorageOperation is the fs storage use-side. Lookups are
// keyed by superblock_id; a missing entry means the caller
// should fall back to the default backend (IRSA / env creds)
// configured on the VFS.
type StorageOperation interface {
	GetBySuperblock(ctx context.Context, superblockID uuid.UUID) (*Storage, error)
}