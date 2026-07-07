package fs

import "context"

// CreateObject registers an S3 object metadata as a
// file-like node. Payload lives in S3; node carries
// metadata. ActionEdit on the parent drive.
func (f *fs) CreateObject(ctx context.Context, driveID, path string, ref ObjectRef, size int64) (Stat, error) {
	return f.doCreate(ctx, driveID, path, NodeKindObject, ref, size)
}
