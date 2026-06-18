// Package s3 provides an S3/MinIO client implementation of the storage interface
// (defined by the consumer in application/vfs, not exported here).
package s3

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// Config for the S3 client.
type Config struct {
	Region       string
	Endpoint     *string // for MinIO
	AccessKey    string
	SecretKey    string
	UsePathStyle bool
	HTTPClient   *http.Client // for testing
}

// Client is an S3/MinIO client.
type Client struct {
	api       *s3.Client
	presigner *s3.PresignClient
}

// NewClient creates a new S3 client.
// If AccessKey and SecretKey are provided they take precedence;
// otherwise the AWS default credential chain is used (IRSA, EC2 role, etc.).
func NewClient(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.Region == "" {
		return nil, errors.New("s3: region is required")
	}

	loadOpts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.Region),
	}
	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		creds := credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")
		loadOpts = append(loadOpts, awsconfig.WithCredentialsProvider(creds))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("s3: load aws config: %w", err)
	}

	api := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != nil && *cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(*cfg.Endpoint)
		}
		o.UsePathStyle = cfg.UsePathStyle
		if cfg.HTTPClient != nil {
			o.HTTPClient = cfg.HTTPClient
		}
	})
	presigner := s3.NewPresignClient(api)

	return &Client{api: api, presigner: presigner}, nil
}

// PutObject uploads an object to the given bucket.
func (c *Client) PutObject(ctx context.Context, bucket, key string, reader io.Reader, _ int64) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("s3: read body: %w", err)
	}
	_, err = c.api.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	})
	return err
}

// DeleteObject removes an object from the given bucket.
func (c *Client) DeleteObject(ctx context.Context, bucket, key string) error {
	_, err := c.api.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return err
}

// DeleteObjects removes multiple objects in a single request.
func (c *Client) DeleteObjects(ctx context.Context, bucket string, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	objects := make([]s3types.ObjectIdentifier, len(keys))
	for i, k := range keys {
		objects[i] = s3types.ObjectIdentifier{Key: aws.String(k)}
	}
	_, err := c.api.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(bucket),
		Delete: &s3types.Delete{
			Objects: objects,
			Quiet:   aws.Bool(true),
		},
	})
	return err
}

// ObjectExists checks whether the object exists in the bucket.
func (c *Client) ObjectExists(ctx context.Context, bucket, key string) (bool, error) {
	_, err := c.api.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var notFound *s3types.NotFound
		if errors.As(err, &notFound) {
			return false, nil
		}
		// 404 also surfaces as a generic NotFound error from the SDK.
		return false, nil
	}
	return true, nil
}

// GetObject downloads an object as a byte slice.
func (c *Client) GetObject(ctx context.Context, bucket, key string) ([]byte, error) {
	out, err := c.api.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = out.Body.Close() }()
	return io.ReadAll(out.Body)
}

// GetObjectSize returns the size of the object in bytes.
func (c *Client) GetObjectSize(ctx context.Context, bucket, key string) (int64, error) {
	out, err := c.api.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return 0, err
	}
	if out.ContentLength == nil {
		return 0, nil
	}
	return *out.ContentLength, nil
}

// GetObjectChecksum returns the ETag of the object.
func (c *Client) GetObjectChecksum(ctx context.Context, bucket, key string) (string, error) {
	out, err := c.api.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return "", err
	}
	if out.ETag == nil {
		return "", nil
	}
	return *out.ETag, nil
}

// GetPresignedUploadURL returns a presigned PUT URL.
func (c *Client) GetPresignedUploadURL(ctx context.Context, bucket, key, _ string, _ int64, _ string, expiry time.Duration) (string, error) {
	req, err := c.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

// GetPresignedDownloadURL returns a presigned GET URL.
func (c *Client) GetPresignedDownloadURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error) {
	req, err := c.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}
