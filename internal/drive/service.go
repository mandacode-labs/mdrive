package drive

import (
	"context"
	"time"

	awss3 "github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/crypto"
	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// Service is the public surface for drive lifecycle. Handlers
// call into this; vfs does not depend on Service.
type Service interface {
	Create(ctx context.Context, ownerID string, name string, description string, cfg *StorageConfig) (*Drive, error)
	Get(ctx context.Context, driveID string) (*Drive, error)
	GetStorage(ctx context.Context, driveID string) (*Storage, error)
	Update(ctx context.Context, driveID string, name string, description string) (*Drive, error)
	SoftDelete(ctx context.Context, driveID string) error
	Restore(ctx context.Context, driveID string) (*Drive, error)
	Purge(ctx context.Context, driveID string) error
	ListByOwner(ctx context.Context, ownerID string) ([]*Drive, error)
	ListDeleted(ctx context.Context, before time.Time, limit int) ([]*Drive, error)
}

type service struct {
	repo      Repository
	encryptor crypto.Encryptor
}

// NewService wires the canonical impl.
func NewService(repo Repository, encryptor crypto.Encryptor) Service {
	return &service{repo: repo, encryptor: encryptor}
}

var _ Service = (*service)(nil)

func (s *service) Create(ctx context.Context, ownerID string, name string, description string, cfg *StorageConfig) (*Drive, error) {
	if name == "" {
		return nil, errorx.New(errorx.KindInvalidArgument, "drive: name is required")
	}
	if ownerID == "" {
		return nil, errorx.New(errorx.KindInvalidArgument, "drive: owner_id is required")
	}
	if cfg == nil {
		return nil, errorx.New(errorx.KindInvalidArgument, "drive: storage config is required")
	}
	ownerULID, err := ulid.Parse(ownerID)
	if err != nil {
		return nil, errorx.New(errorx.KindInvalidArgument, "drive: invalid owner id")
	}
	id := ulid.Make()

	var descPtr *string
	if description != "" {
		descPtr = &description
	}

	d := New(id, name, ownerULID)
	d.SetDescription(descPtr)

	if err := s.repo.Create(ctx, d); err != nil {
		return nil, err
	}

	// Build storage row with encryption.
	storageRow, err := s.buildStorageRow(ctx, id.String(), cfg)
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateStorage(ctx, storageRow); err != nil {
		return nil, err
	}

	created, err := s.repo.UpdateFields(ctx, id, name, descPtr)
	if err != nil {
		return nil, err
	}
	return created, nil
}

// buildStorageRow validates the storage config against the
// real S3/MinIO endpoint, encrypts the secret, and returns
// a *Storage ready to persist.
func (s *service) buildStorageRow(ctx context.Context, driveID string, cfg *StorageConfig) (*Storage, error) {
	// Validate by connecting to the backend.
	awsCfg, err := driveAWSConfig(ctx, cfg.Region, cfg.AccessKey, cfg.SecretKey)
	if err != nil {
		return nil, errorx.Wrap(err, "drive: build aws config")
	}
	api := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if ep := derefStr(cfg.Endpoint); ep != "" {
			o.BaseEndpoint = awss3.String(ep)
		}
		o.UsePathStyle = cfg.UsePathStyle
	})
	if _, err := api.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: awss3.String(cfg.Bucket),
	}); err != nil {
		return nil, errorx.Wrap(err, "drive: validate storage bucket (ping)")
	}

	// Encrypt the secret (only if non-empty).
	encryptedSecret := ""
	if cfg.SecretKey != "" {
		ct, err := s.encryptor.Encrypt([]byte(cfg.SecretKey))
		if err != nil {
			return nil, errorx.Wrap(err, "drive: encrypt storage secret")
		}
		encryptedSecret = string(ct)
	}

	return NewStorage(
		driveID,
		cfg.Bucket,
		cfg.Region,
		cfg.Endpoint,
		cfg.AccessKey,
		encryptedSecret,
		cfg.UsePathStyle,
	), nil
}

// driveAWSConfig assembles the AWS SDK config from region +
// credentials. Both empty → SDK default chain (IRSA / env).
// One set, the other empty → error.
func driveAWSConfig(ctx context.Context, region, accessKey, secretKey string) (awss3.Config, error) {
	if region == "" {
		return awss3.Config{}, errDriveMissing("region is required")
	}
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(region),
	}
	hasAK := accessKey != ""
	hasSK := secretKey != ""
	if hasAK != hasSK {
		return awss3.Config{}, errDriveMissing("access_key and secret_key must be both set or both empty")
	}
	if hasAK {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		))
	}
	return awsconfig.LoadDefaultConfig(ctx, opts...)
}

type driveConfigError struct{ msg string }

func (e *driveConfigError) Error() string { return "drive: " + e.msg }
func errDriveMissing(msg string) error    { return &driveConfigError{msg: msg} }

func (s *service) Get(ctx context.Context, driveID string) (*Drive, error) {
	id, err := ulid.Parse(driveID)
	if err != nil {
		return nil, errorx.Wrap(err, "drive: invalid drive id", errorx.KindInvalidArgument)
	}
	return s.repo.Read(ctx, id)
}

func (s *service) GetStorage(ctx context.Context, driveID string) (*Storage, error) {
	return s.repo.ReadStorage(ctx, driveID)
}

func (s *service) Update(ctx context.Context, driveID string, name string, description string) (*Drive, error) {
	id, err := ulid.Parse(driveID)
	if err != nil {
		return nil, errorx.Wrap(err, "drive: invalid drive id", errorx.KindInvalidArgument)
	}
	if err != nil {
		return nil, err
	}
	existing, err := s.repo.Read(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errorx.New(errorx.KindNotFound, "drive: not found")
	}
	newName := existing.Name()
	if name != "" {
		newName = name
	}
	var descPtr *string
	if description != "" {
		descPtr = &description
	}
	return s.repo.UpdateFields(ctx, id, newName, descPtr)
}

func (s *service) SoftDelete(ctx context.Context, driveID string) error {
	id, err := ulid.Parse(driveID)
	if err != nil {
		return errorx.Wrap(err, "drive: invalid drive id", errorx.KindInvalidArgument)
	}
	if _, err := s.repo.Read(ctx, id); err != nil {
		return err
	}
	return s.repo.SoftDelete(ctx, id, time.Now())
}

func (s *service) Restore(ctx context.Context, driveID string) (*Drive, error) {
	id, err := ulid.Parse(driveID)
	if err != nil {
		return nil, errorx.Wrap(err, "drive: invalid drive id", errorx.KindInvalidArgument)
	}
	if err != nil {
		return nil, err
	}
	existing, err := s.repo.Read(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.DeletedAt() == nil {
		return nil, errorx.New(errorx.KindFailedPrecondition, "drive: not deleted")
	}
	if err := s.repo.Restore(ctx, id); err != nil {
		return nil, err
	}
	return s.repo.Read(ctx, id)
}

func (s *service) Purge(ctx context.Context, driveID string) error {
	id, err := ulid.Parse(driveID)
	if err != nil {
		return errorx.Wrap(err, "drive: invalid drive id", errorx.KindInvalidArgument)
	}
	return s.repo.Destroy(ctx, id)
}

func (s *service) ListByOwner(ctx context.Context, ownerID string) ([]*Drive, error) {
	if ownerID == "" {
		return nil, errorx.New(errorx.KindInvalidArgument, "drive: owner_id is required")
	}
	return s.repo.ListByOwner(ctx, ownerID)
}

func (s *service) ListDeleted(ctx context.Context, before time.Time, limit int) ([]*Drive, error) {
	if limit <= 0 {
		limit = 1000
	}
	return s.repo.ListDeleted(ctx, before, limit)
}