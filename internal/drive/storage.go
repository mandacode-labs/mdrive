package drive

// Provider represents a storage backend type.
type Provider string

const (
	ProviderS3    Provider = "s3"
	ProviderMinio Provider = "minio"
)

func (p Provider) String() string { return string(p) }

// Storage is the per-drive S3/MinIO backend configuration.
// Separated from Drive for security (sensitive fields) and
// table-size efficiency (drive row stays small).
type Storage struct {
	driveID           string
	provider          Provider
	bucket            string
	endpoint          *string
	region            string
	accessKey         string
	encryptedSecret   string
	usePathStyle      bool
}

func NewStorage(
	driveID string,
	bucket, region string,
	endpoint *string,
	accessKey, encryptedSecret string,
	usePathStyle bool,
) *Storage {
	return &Storage{
		driveID:         driveID,
		provider:        ProviderS3,
		bucket:          bucket,
		endpoint:        endpoint,
		region:          region,
		accessKey:       accessKey,
		encryptedSecret: encryptedSecret,
		usePathStyle:    usePathStyle,
	}
}

func (s *Storage) DriveID() string         { return s.driveID }
func (s *Storage) Provider() Provider       { return s.provider }
func (s *Storage) Bucket() string           { return s.bucket }
func (s *Storage) Endpoint() *string        { return s.endpoint }
func (s *Storage) Region() string           { return s.region }
func (s *Storage) AccessKey() string        { return s.accessKey }
func (s *Storage) EncryptedSecret() string  { return s.encryptedSecret }
func (s *Storage) UsePathStyle() bool      { return s.usePathStyle }

// StorageConfig is the input form of a Storage (used by
// CreateCommand-style operations from the handler).
type StorageConfig struct {
	Bucket            string
	Endpoint          *string
	Region            string
	AccessKey         string
	SecretKey         string
	UsePathStyle      bool
}