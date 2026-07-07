package provider

import (
	"context"
	"errors"
	"time"

	awss3 "github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// NewS3Provider returns a StorageProvider backed by the AWS
// S3 SDK. The config determines the bucket, region, and
// credentials; endpoint + usePathStyle control MinIO-style
// custom deployments.
func NewS3Provider(cfg *ProviderConfig) StorageProvider {
	return &s3Provider{cfg: cfg}
}

type s3Provider struct {
	cfg *ProviderConfig
}

func (p *s3Provider) buildClient(ctx context.Context) (*s3.Client, error) {
	if p.cfg == nil {
		return nil, errorx.New(errorx.KindInvalidArgument, "s3: provider config is nil")
	}
	if p.cfg.Region == "" {
		return nil, errorx.New(errorx.KindInvalidArgument, "s3: region is required")
	}
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(p.cfg.Region),
	}
	hasAK := p.cfg.AccessKey != ""
	hasSK := p.cfg.SecretKey != ""
	if hasAK != hasSK {
		return nil, errorx.New(errorx.KindInvalidArgument, "s3: access_key and secret_key must be both set or both empty")
	}
	if hasAK {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(p.cfg.AccessKey, p.cfg.SecretKey, ""),
		))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, errorx.Wrap(err, "s3: load aws config")
	}
	return s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if p.cfg.Endpoint != "" {
			o.BaseEndpoint = awss3.String(p.cfg.Endpoint)
		}
		if p.cfg.UsePathStyle {
			o.UsePathStyle = true
		}
	}), nil
}

func (p *s3Provider) bucket() (string, error) {
	if p.cfg == nil || p.cfg.Bucket == "" {
		return "", errorx.New(errorx.KindInvalidArgument, "s3: bucket is required")
	}
	return p.cfg.Bucket, nil
}

func (p *s3Provider) Ping(ctx context.Context) error {
	client, err := p.buildClient(ctx)
	if err != nil {
		return err
	}
	bucket, err := p.bucket()
	if err != nil {
		return err
	}
	_, err = client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: awss3.String(bucket),
	})
	if err != nil {
		return errorx.Wrap(err, "s3: ping bucket")
	}
	return nil
}

func (p *s3Provider) PresignUpload(ctx context.Context, key, contentType string, expiry time.Duration) (UploadInfo, error) {
	client, err := p.buildClient(ctx)
	if err != nil {
		return UploadInfo{}, err
	}
	bucket, err := p.bucket()
	if err != nil {
		return UploadInfo{}, err
	}
	req, err := s3.NewPresignClient(client).PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      awss3.String(bucket),
		Key:         awss3.String(key),
		ContentType: awss3.String(contentType),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return UploadInfo{}, err
	}
	return UploadInfo{URL: req.URL, Key: key}, nil
}

func (p *s3Provider) PresignDownload(ctx context.Context, key string, expiry time.Duration) (string, error) {
	client, err := p.buildClient(ctx)
	if err != nil {
		return "", err
	}
	bucket, err := p.bucket()
	if err != nil {
		return "", err
	}
	req, err := s3.NewPresignClient(client).PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: awss3.String(bucket),
		Key:    awss3.String(key),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

func (p *s3Provider) HeadObject(ctx context.Context, key string) (ObjectMetadata, error) {
	client, err := p.buildClient(ctx)
	if err != nil {
		return ObjectMetadata{}, err
	}
	bucket, err := p.bucket()
	if err != nil {
		return ObjectMetadata{}, err
	}
	out, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: awss3.String(bucket),
		Key:    awss3.String(key),
	})
	if err != nil {
		var notFound *s3types.NotFound
		if errors.As(err, &notFound) {
			return ObjectMetadata{}, errorx.New(errorx.KindNotFound, "s3: object not found")
		}
		return ObjectMetadata{}, errorx.Wrap(err, "s3: head object")
	}
	md := ObjectMetadata{Bucket: bucket}
	if out.ContentLength != nil {
		md.Size = *out.ContentLength
	}
	if out.ETag != nil {
		md.ETag = *out.ETag
	}
	if out.LastModified != nil {
		md.MTime = *out.LastModified
	}
	return md, nil
}