package s3

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// UploadInfo is the result of a presigned PUT (URL + key).
type UploadInfo struct {
	URL string
	Key string
}

// ObjectMetadata is the result of HeadObject (size + ETag + mtime).
type ObjectMetadata struct {
	Bucket string
	Size   int64
	ETag   string
	MTime  time.Time
}

// ErrNotFound indicates a missing S3 object.
var ErrNotFound = errors.New("s3: object not found")

// PresignDownload returns a presigned GET URL for the object.
func PresignDownload(ctx context.Context, api *s3.Client, bucket, key string, expiry time.Duration) (string, error) {
	if api == nil {
		return "", errors.New("s3: client is required")
	}
	req, err := s3.NewPresignClient(api).PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

// PresignUpload returns a presigned PUT URL and the key the
// caller must echo back to fs.Service.Complete.
func PresignUpload(ctx context.Context, api *s3.Client, bucket, key, contentType string, expiry time.Duration) (UploadInfo, error) {
	if api == nil {
		return UploadInfo{}, errors.New("s3: client is required")
	}
	req, err := s3.NewPresignClient(api).PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return UploadInfo{}, err
	}
	return UploadInfo{URL: req.URL, Key: key}, nil
}

// HeadObject returns the backend-reported metadata for an
// existing object. Returns ErrNotFound if missing.
func HeadObject(ctx context.Context, api *s3.Client, bucket, key string) (ObjectMetadata, error) {
	if api == nil {
		return ObjectMetadata{}, errors.New("s3: client is required")
	}
	out, err := api.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var notFound *s3types.NotFound
		if errors.As(err, &notFound) {
			return ObjectMetadata{}, ErrNotFound
		}
		return ObjectMetadata{}, err
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

// DeleteObject removes an object.
func DeleteObject(ctx context.Context, api *s3.Client, bucket, key string) error {
	if api == nil {
		return errors.New("s3: client is required")
	}
	_, err := api.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return err
}