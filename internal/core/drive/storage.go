package drive

// Storage holds the S3/MinIO backend configuration for a drive,
// including the per-drive wrapped data encryption key. Separated
// from Drive for security (sensitive fields) and table-size efficiency.
type Storage struct {
	driveID      string
	bucket       string
	endpoint     *string
	region       string
	accessKey    string
	secretKey    string
	usePathStyle bool
	wrappedDEK   string
}

// StorageConfig is the input form of a Storage (used by CreateCommand).
type StorageConfig struct {
	Bucket       string
	Endpoint     *string
	Region       string
	AccessKey    string
	SecretKey    string
	UsePathStyle bool
}

// NewStorage creates a new Storage. wrappedDEK is the per-drive data
// encryption key wrapped with the master key (base64 string). Pass an
// empty string if the drive was created before DEK support.
func NewStorage(
	driveID string,
	bucket string,
	endpoint *string,
	region string,
	accessKey string,
	secretKey string,
	usePathStyle bool,
	wrappedDEK string,
) *Storage {
	return &Storage{
		driveID:      driveID,
		bucket:       bucket,
		endpoint:     endpoint,
		region:       region,
		accessKey:    accessKey,
		secretKey:    secretKey,
		usePathStyle: usePathStyle,
		wrappedDEK:   wrappedDEK,
	}
}

// Getters.
func (s *Storage) DriveID() string    { return s.driveID }
func (s *Storage) Bucket() string     { return s.bucket }
func (s *Storage) Endpoint() *string  { return s.endpoint }
func (s *Storage) Region() string     { return s.region }
func (s *Storage) AccessKey() string  { return s.accessKey }
func (s *Storage) SecretKey() string  { return s.secretKey }
func (s *Storage) UsePathStyle() bool { return s.usePathStyle }

// WrappedDEK returns the per-drive data encryption key wrapped with
// the master key. Empty for drives created before DEK support.
func (s *Storage) WrappedDEK() string { return s.wrappedDEK }
