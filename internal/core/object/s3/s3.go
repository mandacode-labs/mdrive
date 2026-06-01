package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	appconfig "github.com/mandacode-labs/retrowin-go/internal/config"
	"github.com/mandacode-labs/retrowin-go/internal/core/object"
	apperrors "github.com/mandacode-labs/retrowin-go/internal/errors"
)

// S3Storage implements the object.Storage interface using AWS S3.
type S3Storage struct {
	client        *s3.Client
	presigner     *s3.PresignClient
	defaultBucket string
	keyPrefix     string
}

// New creates a new S3 storage instance.
func New(cfg *appconfig.StorageConfig) (object.Storage, error) {
	if cfg == nil {
		return nil, fmt.Errorf("storage config is required")
	}

	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.Region),
	}
	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true
		}
	})

	presigner := s3.NewPresignClient(client)

	return &S3Storage{
		client:        client,
		presigner:     presigner,
		defaultBucket: cfg.Bucket,
		keyPrefix:     cfg.StorageKeyPrefix(),
	}, nil
}

// DefaultBucket returns the configured default bucket name.
func (s *S3Storage) DefaultBucket() string {
	return s.defaultBucket
}

// KeyPrefix returns the configured prefix for storage keys.
func (s *S3Storage) KeyPrefix() string {
	return s.keyPrefix
}

func (s *S3Storage) resolveBucket(bucket string) string {
	if bucket != "" {
		return bucket
	}
	return s.defaultBucket
}

// PutObject streams data directly to S3.
func (s *S3Storage) PutObject(ctx context.Context, bucket string, key string, reader io.Reader, size int64) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.resolveBucket(bucket)),
		Key:           aws.String(key),
		Body:          reader,
		ContentLength: aws.Int64(size),
	})
	if err != nil {
		return fmt.Errorf("failed to put object: %w", err)
	}
	return nil
}

// GetPresignedDownloadURL generates a presigned URL for direct client download.
func (s *S3Storage) GetPresignedDownloadURL(ctx context.Context, bucket string, key string, expiry time.Duration) (string, error) {
	req, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.resolveBucket(bucket)),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned download URL: %w", err)
	}
	return req.URL, nil
}

// GetPresignedUploadURL generates a presigned URL for direct client upload.
// ContentLength is enforced to prevent size mismatches between declared and actual upload.
func (s *S3Storage) GetPresignedUploadURL(ctx context.Context, bucket string, key string, contentType string, size int64, expiry time.Duration) (string, error) {
	input := &s3.PutObjectInput{
		Bucket: aws.String(s.resolveBucket(bucket)),
		Key:    aws.String(key),
	}
	if size > 0 {
		input.ContentLength = aws.Int64(size)
	}
	if contentType != "" {
		input.ContentType = aws.String(contentType)
	}

	req, err := s.presigner.PresignPutObject(ctx, input, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned upload URL: %w", err)
	}
	return req.URL, nil
}

// DeleteObject removes an object from storage.
func (s *S3Storage) DeleteObject(ctx context.Context, bucket string, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.resolveBucket(bucket)),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}
	return nil
}

// ObjectExists checks if an object exists in storage.
func (s *S3Storage) ObjectExists(ctx context.Context, bucket string, key string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.resolveBucket(bucket)),
		Key:    aws.String(key),
	})
	if err != nil {
		var notFound *types.NotFound
		if errors.As(err, &notFound) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check object existence: %w", err)
	}
	return true, nil
}

// GetObjectSize returns the size of an object in bytes.
func (s *S3Storage) GetObjectSize(ctx context.Context, bucket string, key string) (int64, error) {
	resp, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.resolveBucket(bucket)),
		Key:    aws.String(key),
	})
	if err != nil {
		var notFound *types.NotFound
		if errors.As(err, &notFound) {
			return 0, apperrors.NotFound("object not found")
		}
		return 0, fmt.Errorf("failed to get object info: %w", err)
	}
	if resp.ContentLength == nil {
		return 0, nil
	}
	return *resp.ContentLength, nil
}
