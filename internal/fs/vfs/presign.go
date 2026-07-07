package vfs

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	awss3 "github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
)

// vfs is the concrete implementation of fs.VFS.
type vfs struct {
	nodeOp    fs.NodeOperation
	superop   fs.SuperOperation
	storageOp fs.StorageOperation
	// defaultS3 is the app-level default *s3.Client (IRSA
	// fallback). nil = SDK default chain at call time.
	defaultS3 *s3.Client
}

// Config groups the dependencies of vfs.
type Config struct {
	NodeOp    fs.NodeOperation
	SuperOp   fs.SuperOperation
	StorageOp fs.StorageOperation
	DefaultS3 *s3.Client
}

// New constructs an fs.VFS implementation.
func New(cfg Config) fs.VFS {
	return &vfs{
		nodeOp:    cfg.NodeOp,
		superop:   cfg.SuperOp,
		storageOp: cfg.StorageOp,
		defaultS3: cfg.DefaultS3,
	}
}

func strDeref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// buildClient returns an *s3.Client for the given storage.
// nil storage → returns v.defaultS3 (caller should fall
// back to SDK default if defaultS3 is also nil).
func (v *vfs) buildClient(ctx context.Context, storage *fs.Storage) (*s3.Client, string, error) {
	if storage == nil {
		return v.defaultS3, "", nil
	}
	awsCfg, err := buildAWSConfig(ctx, storage.Region(), storage.AccessKey(), storage.SecretKey())
	if err != nil {
		return nil, "", err
	}
	api := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if ep := strDeref(storage.Endpoint()); ep != "" {
			o.BaseEndpoint = awss3.String(ep)
		}
		o.UsePathStyle = storage.UsePathStyle()
	})
	return api, storage.Bucket(), nil
}

// buildAWSConfig assembles the AWS SDK config from region +
// credentials. Both empty → SDK default chain (IRSA / env).
// One set, the other empty → error.
func buildAWSConfig(ctx context.Context, region, accessKey, secretKey string) (awss3.Config, error) {
	if region == "" {
		return awss3.Config{}, errorx.New(errorx.KindInvalidArgument, "s3: region is required")
	}
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(region),
	}
	hasAK := accessKey != ""
	hasSK := secretKey != ""
	if hasAK != hasSK {
		return awss3.Config{}, errorx.New(errorx.KindInvalidArgument, "s3: access_key and secret_key must be both set or both empty")
	}
	if hasAK {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		))
	}
	return awsconfig.LoadDefaultConfig(ctx, opts...)
}

// resolveBackend returns the S3 client + bucket for a given
// superblock. Lookup order:
//  1. per-superblock Storage
//  2. app-level default S3 client
func (v *vfs) resolveBackend(ctx context.Context, sbID uuid.UUID) (*s3.Client, string, error) {
	storage, err := v.storageOp.GetBySuperblock(ctx, sbID)
	if err != nil {
		return nil, "", err
	}
	if storage != nil {
		api, bucket, err := v.buildClient(ctx, storage)
		if err != nil {
			return nil, "", err
		}
		return api, bucket, nil
	}
	return v.defaultS3, "", nil
}

// Download returns a presigned GET URL for an object-kind node.
func (v *vfs) Download(ctx context.Context, dentry *fs.Dentry, expiry time.Duration) (string, error) {
	if dentry.Node.Kind() != fs.NodeKindObject {
		return "", errorx.New(errorx.KindInvalidArgument, "vfs: download target is not an object")
	}
	var oc fs.ObjectContent
	if err := json.Unmarshal(dentry.Node.Data(), &oc); err != nil {
		return "", errorx.Wrap(err, "vfs: object content", errorx.KindInternal)
	}
	client, bucket, err := v.resolveBackend(ctx, dentry.Node.SuperblockID())
	if err != nil {
		return "", err
	}
	req, err := s3.NewPresignClient(client).PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: awss3.String(bucket),
		Key:    awss3.String(oc.Key),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

// Upload returns a presigned PUT URL for a new object.
func (v *vfs) Upload(ctx context.Context, parent *fs.Dentry, key, contentType string, expiry time.Duration) (fs.UploadInfo, error) {
	client, bucket, err := v.resolveBackend(ctx, parent.Node.SuperblockID())
	if err != nil {
		return fs.UploadInfo{}, err
	}
	req, err := s3.NewPresignClient(client).PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      awss3.String(bucket),
		Key:         awss3.String(key),
		ContentType: awss3.String(contentType),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return fs.UploadInfo{}, err
	}
	return fs.UploadInfo{URL: req.URL, Key: key}, nil
}

// Verify returns the backend-reported metadata for an
// existing object-kind node.
func (v *vfs) Verify(ctx context.Context, dentry *fs.Dentry) (fs.ObjectMetadata, error) {
	if dentry.Node.Kind() != fs.NodeKindObject {
		return fs.ObjectMetadata{}, errorx.New(errorx.KindInvalidArgument, "vfs: verify target is not an object")
	}
	var oc fs.ObjectContent
	if err := json.Unmarshal(dentry.Node.Data(), &oc); err != nil {
		return fs.ObjectMetadata{}, errorx.Wrap(err, "vfs: object content", errorx.KindInternal)
	}
	client, bucket, err := v.resolveBackend(ctx, dentry.Node.SuperblockID())
	if err != nil {
		return fs.ObjectMetadata{}, err
	}
	out, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: awss3.String(bucket),
		Key:    awss3.String(oc.Key),
	})
	if err != nil {
		return fs.ObjectMetadata{}, mapS3Err(err, "head object")
	}
	return toFsObjectMetadata(bucket, out)
}

// VerifyByKey returns backend metadata for a key under a
// specific superblock. Used by Service.Complete.
func (v *vfs) VerifyByKey(ctx context.Context, sbID uuid.UUID, key string) (fs.ObjectMetadata, error) {
	client, bucket, err := v.resolveBackend(ctx, sbID)
	if err != nil {
		return fs.ObjectMetadata{}, err
	}
	out, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: awss3.String(bucket),
		Key:    awss3.String(key),
	})
	if err != nil {
		return fs.ObjectMetadata{}, mapS3Err(err, "head object by key")
	}
	return toFsObjectMetadata(bucket, out)
}

func toFsObjectMetadata(bucket string, out *s3.HeadObjectOutput) (fs.ObjectMetadata, error) {
	md := fs.ObjectMetadata{Bucket: bucket}
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

// mapS3Err maps an s3 SDK error to an errorx error. Handles
// NotFound specifically; everything else is wrapped.
func mapS3Err(err error, msg string) error {
	var notFound *s3types.NotFound
	if errors.As(err, &notFound) {
		return errorx.New(errorx.KindNotFound, "s3: "+msg+" (not found)")
	}
	return errorx.Wrap(err, "s3: "+msg)
}