package vfs

import (
	"context"
	"encoding/json"
	"time"

	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
	"github.com/mandacode-labs/mdrive/internal/provider/s3"
)

func strDeref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// resolveBackend returns the S3 client + bucket for a given
// superblock. Lookup order:
//  1. per-superblock Storage (if set)
//  2. app-level default Storage (if set)
//  3. nil client + empty bucket = SDK default chain (IRSA)
//     — caller should fail at presign time if it needs creds.
func (v *vfs) resolveBackend(ctx context.Context, sbID uuid.UUID) (*awss3.Client, string, error) {
	storage, err := v.storageOp.GetBySuperblock(ctx, sbID)
	if err != nil {
		return nil, "", err
	}
	if storage == nil {
		storage = v.defaultStorage
	}
	if storage == nil {
		// No per-drive config and no default — fall back to
		// SDK default credential chain (IRSA / env).
		return nil, "", nil
	}
	client, err := s3.BuildClient(ctx, storage.Region(), storage.AccessKey(), storage.SecretKey(), strDeref(storage.Endpoint()), storage.UsePathStyle())
	if err != nil {
		return nil, "", err
	}
	return client, storage.Bucket(), nil
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
	return s3.PresignDownload(ctx, client, bucket, oc.Key, expiry)
}

// Upload returns a presigned PUT URL for a new object at
// parent/name. The caller PUTs to URL and then calls
// Service.Complete with the returned Key to finalize the node.
func (v *vfs) Upload(ctx context.Context, parent *fs.Dentry, key, contentType string, expiry time.Duration) (fs.UploadInfo, error) {
	client, bucket, err := v.resolveBackend(ctx, parent.Node.SuperblockID())
	if err != nil {
		return fs.UploadInfo{}, err
	}
	info, err := s3.PresignUpload(ctx, client, bucket, key, contentType, expiry)
	if err != nil {
		return fs.UploadInfo{}, err
	}
	return fs.UploadInfo{URL: info.URL, Key: info.Key}, nil
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
	return toFsObjectMetadata(s3.HeadObject(ctx, client, bucket, oc.Key))
}

// VerifyByKey returns backend metadata for a key under a
// specific superblock. Used by Service.Complete (the node
// doesn't exist yet).
func (v *vfs) VerifyByKey(ctx context.Context, sbID uuid.UUID, key string) (fs.ObjectMetadata, error) {
	client, bucket, err := v.resolveBackend(ctx, sbID)
	if err != nil {
		return fs.ObjectMetadata{}, err
	}
	return toFsObjectMetadata(s3.HeadObject(ctx, client, bucket, key))
}

func toFsObjectMetadata(m s3.ObjectMetadata, err error) (fs.ObjectMetadata, error) {
	if err != nil {
		return fs.ObjectMetadata{}, err
	}
	return fs.ObjectMetadata{
		Bucket: m.Bucket,
		Size:   m.Size,
		ETag:   m.ETag,
		MTime:  m.MTime,
	}, nil
}