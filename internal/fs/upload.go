package fs

import (
	"context"
	"path"
	"time"

	"github.com/google/uuid"
)

// Download returns a presigned GET URL for an object-kind node
// at path. ActionRead.
func (f *fs) Download(ctx context.Context, driveID, path string, expiry time.Duration) (string, error) {
	dentry, err := f.walkForKind(ctx, driveID, path, NodeKindObject)
	if err != nil {
		return "", err
	}
	if err := f.requireRead(ctx, dentry.DriveID); err != nil {
		return "", err
	}
	return f.vfs.Download(ctx, dentry, expiry)
}

// Upload returns a presigned PUT URL for a new object at path.
// The returned Key must be passed to Complete after the PUT.
// ActionWrite + ActionUpload on parent drive.
func (f *fs) Upload(ctx context.Context, driveID, path, contentType string, expiry time.Duration) (UploadInfo, error) {
	parent, _, err := f.doPathParent(ctx, driveID, path)
	if err != nil {
		return UploadInfo{}, err
	}
	if err := f.requireWrite(ctx, parent.DriveID); err != nil {
		return UploadInfo{}, err
	}
	if err := f.requireUpload(ctx, parent.DriveID); err != nil {
		return UploadInfo{}, err
	}
	key := generateUploadKey(driveID)
	return f.vfs.Upload(ctx, parent, key, contentType, expiry)
}

// Complete verifies the S3 object exists (size from backend)
// and creates the object-kind node at path. ActionUpload.
func (f *fs) Complete(ctx context.Context, driveID, path, key string) (Stat, error) {
	parent, name, err := f.doPathParent(ctx, driveID, path)
	if err != nil {
		return Stat{}, err
	}
	if err := f.requireUpload(ctx, parent.DriveID); err != nil {
		return Stat{}, err
	}

	meta, err := f.vfs.VerifyByKey(ctx, parent.Node.SuperblockID(), key)
	if err != nil {
		return Stat{}, err
	}

	oc := &ObjectContent{
		Bucket: meta.Bucket,
		Key:    key,
		Size:   meta.Size,
	}
	_ = meta.ETag // could store as Checksum later

	data, err := oc.Marshal()
	if err != nil {
		return Stat{}, err
	}
	node := NewNode(uuid.New(), parent.Node.SuperblockID(), NodeKindObject)
	if err := node.Write(data, meta.Size); err != nil {
		return Stat{}, err
	}
	if err := f.vfs.Create(ctx, parent, node, name); err != nil {
		return Stat{}, err
	}
	return NodeToStat(node), nil
}

// generateUploadKey builds the S3 key for an upload-in-progress.
// Path: drives/{driveID}/uploads/{uuid}. Collision-safe (random
// uuid per call) and easy for GC to sweep.
func generateUploadKey(driveID string) string {
	return path.Join("drives", driveID, "uploads", uuid.NewString())
}