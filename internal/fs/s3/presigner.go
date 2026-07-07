package s3

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
)

// Presigner is the fs presign surface backed by the AWS S3 SDK.
type Presigner interface {
	Bucket() string
	PresignDownload(ctx context.Context, key string, expiry time.Duration) (string, error)
	PresignUpload(ctx context.Context, key, contentType string, expiry time.Duration) (fs.UploadInfo, error)
	ObjectMetadata(ctx context.Context, key string) (fs.ObjectMetadata, error)
	DeleteObject(ctx context.Context, key string) error
}

// presigner is the S3/MinIO impl. One instance per bucket —
// the parent vfs layer caches per superblock.
type presigner struct {
	bucket string // empty for the default IRSA presigner
	api    *s3.Client
}

func (p *presigner) Bucket() string { return p.bucket }

func (p *presigner) PresignDownload(ctx context.Context, key string, expiry time.Duration) (string, error) {
	if p.api == nil {
		return "", errorx.New(errorx.KindUnavailable, "s3: client not initialized")
	}
	req, err := s3.NewPresignClient(p.api).PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

func (p *presigner) PresignUpload(ctx context.Context, key, contentType string, expiry time.Duration) (fs.UploadInfo, error) {
	if p.api == nil {
		return fs.UploadInfo{}, errorx.New(errorx.KindUnavailable, "s3: client not initialized")
	}
	req, err := s3.NewPresignClient(p.api).PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(p.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return fs.UploadInfo{}, err
	}
	return fs.UploadInfo{
		URL: req.URL,
		Key: key,
	}, nil
}

func (p *presigner) ObjectMetadata(ctx context.Context, key string) (fs.ObjectMetadata, error) {
	if p.api == nil {
		return fs.ObjectMetadata{}, errorx.New(errorx.KindUnavailable, "s3: client not initialized")
	}
	out, err := p.api.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var notFound *s3types.NotFound
		if errors.As(err, &notFound) {
			return fs.ObjectMetadata{}, errorx.New(errorx.KindNotFound, "s3: object not found")
		}
		var httpErr *awshttp.ResponseError
		if errors.As(err, &httpErr) && httpErr.Response != nil && httpErr.Response.StatusCode == 404 {
			return fs.ObjectMetadata{}, errorx.New(errorx.KindNotFound, "s3: object not found")
		}
		return fs.ObjectMetadata{}, errorx.Wrap(err, "s3: head object")
	}
	md := fs.ObjectMetadata{Bucket: p.bucket}
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

func (p *presigner) DeleteObject(ctx context.Context, key string) error {
	if p.api == nil {
		return errorx.New(errorx.KindUnavailable, "s3: client not initialized")
	}
	_, err := p.api.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(key),
	})
	return err
}