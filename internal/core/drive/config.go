package drive

// StorageConfig holds the S3/MinIO backend configuration (input to CreateCommand).
type StorageConfig struct {
	Bucket       string
	Endpoint     *string
	Region       string
	AccessKey    string
	SecretKey    string
	UsePathStyle bool
}

// CreateCommand for creating a new drive.
type CreateCommand struct {
	Name        string
	Description *string
	Provider    Provider
	OwnerID     string
	Storage     StorageConfig
}

// UpdateCommand for updating a drive's mutable fields.
type UpdateCommand struct {
	Name        *string
	Description *string
}
