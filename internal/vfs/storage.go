package vfs

// Provider represents a storage backend type.
type Provider string

const (
	ProviderS3    Provider = "s3"
	ProviderMinio Provider = "minio"
)

type Storage struct {
	driveID      string
	provider     Provider
	bucket       string
	endpoint     *string
	region       string
	accessKey    string
	secretKey    string
	usePathStyle bool
}

func NewStorage(
	driveID string,
	bucket string,
	endpoint *string,
	region string,
	accessKey string,
	secretKey string,
	usePathStyle bool,
) *Storage {
	return &Storage{
		driveID:      driveID,
		bucket:       bucket,
		endpoint:     endpoint,
		region:       region,
		accessKey:    accessKey,
		secretKey:    secretKey,
		usePathStyle: usePathStyle,
	}
}

func (s *Storage) DriveID() string    { return s.driveID }
func (s *Storage) Provider() Provider { return s.provider }
func (s *Storage) Bucket() string     { return s.bucket }
func (s *Storage) Endpoint() *string  { return s.endpoint }
func (s *Storage) Region() string     { return s.region }
func (s *Storage) AccessKey() string  { return s.accessKey }
func (s *Storage) SecretKey() string  { return s.secretKey }
func (s *Storage) UsePathStyle() bool { return s.usePathStyle }
