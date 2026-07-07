package s3

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// BuildClient builds an *s3.Client from the given storage fields.
// All string params are empty-string = unset. Rules:
//   - region: required (empty → error)
//   - accessKey/secretKey: both empty (IRSA / env) OR both set
//     (static). Mixed → error.
//   - endpoint: empty = SDK default (AWS); non-empty = custom
//     (MinIO, S3-compatible)
//   - usePathStyle: false = virtual-hosted (S3 default);
//     true = path-style (MinIO)
func BuildClient(ctx context.Context, region, accessKey, secretKey, endpoint string, usePathStyle bool) (*s3.Client, error) {
	if region == "" {
		return nil, errors.New("s3: region is required")
	}
	hasAK := accessKey != ""
	hasSK := secretKey != ""
	if hasAK != hasSK {
		return nil, errors.New("s3: access_key and secret_key must be both set or both empty (use IRSA default chain if both empty)")
	}

	loadOpts := []func(*config.LoadOptions) error{
		config.WithRegion(region),
	}
	if hasAK {
		loadOpts = append(loadOpts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("s3: load aws config (region=%s): %w", region, err)
	}

	api := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
		o.UsePathStyle = usePathStyle
	})
	return api, nil
}

// PingBucket verifies that the storage backend is reachable
// and the bucket is accessible with the given credentials.
// Used by drive.Service to validate user-supplied storage
// configs at create time.
func PingBucket(ctx context.Context, api *s3.Client, bucket string) error {
	if bucket == "" {
		return errors.New("s3: bucket is required for ping")
	}
	_, err := api.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		return fmt.Errorf("s3: ping bucket %q: %w", bucket, err)
	}
	return nil
}