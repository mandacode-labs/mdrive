package vfs

type Storage struct {
	driveID      string
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
func (s *Storage) Bucket() string     { return s.bucket }
func (s *Storage) Endpoint() *string  { return s.endpoint }
func (s *Storage) Region() string     { return s.region }
func (s *Storage) AccessKey() string  { return s.accessKey }
func (s *Storage) SecretKey() string  { return s.secretKey }
func (s *Storage) UsePathStyle() bool { return s.usePathStyle }
