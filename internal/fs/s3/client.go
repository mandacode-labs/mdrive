package s3

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
)

// BuildClient builds an *s3.Client for the given storage config.
// If accessKey/secretKey are empty, the AWS SDK default
// credential chain is used (IRSA on EKS, EC2 role, env vars).
func BuildClient(ctx context.Context, region string, accessKey, secretKey string, endpoint *string, usePathStyle bool) (*s3.Client, error) {
	if region == "" {
		return nil, errors.New("s3: region is required")
	}

	loadOpts := []func(*config.LoadOptions) error{
		config.WithRegion(region),
	}
	if accessKey != "" && secretKey != "" {
		loadOpts = append(loadOpts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("s3: load aws config (region=%s)", region))
	}

	api := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if endpoint != nil && *endpoint != "" {
			o.BaseEndpoint = aws.String(*endpoint)
		}
		o.UsePathStyle = usePathStyle
	})
	return api, nil
}

// NewDefaultClient builds the IRSA-fallback client. region is
// the only required field; everything else comes from the AWS
// SDK default credential chain.
func NewDefaultClient(ctx context.Context, region string) (*s3.Client, error) {
	return BuildClient(ctx, region, "", "", nil, false)
}

// NewPresigner returns a Presigner bound to a specific bucket
// and storage config. Returns fs.StorageErrUnavailable if
// the SDK client fails to build (caller should fall back).
func NewPresigner(s *fs.Storage) (Presigner, error) {
	api, err := BuildClient(context.Background(), s.Region(), s.AccessKey(), s.SecretKey(), s.Endpoint(), s.UsePathStyle())
	if err != nil {
		return nil, err
	}
	return &presigner{bucket: s.Bucket(), api: api}, nil
}

// NewDefaultPresigner returns a Presigner that uses the default
// IRSA client. The bucket is set per-presign call (caller
// supplies it on the PresignInfo).
func NewDefaultPresigner(defaultClient *s3.Client) Presigner {
	return &presigner{api: defaultClient}
}