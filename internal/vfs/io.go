package vfs

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs/permission"
)

// Read returns the inline data of a file-kind node. Object-kind
// nodes return an error (use upload.PresignDownload for those).
// Linux vfs_read.
func (v *vfs) Read(ctx context.Context, driveID string, path string) ([]byte, error) {
	dentry, err := v.resolveTarget(ctx, driveID, path, permission.ActionView)
	if err != nil {
		return nil, err
	}
	if dentry.Node.Kind() != NodeKindFile {
		return nil, errorx.New(errorx.KindInvalidArgument, "vfs: not a file")
	}
	return dentry.Node.Data(), nil
}

// Write creates or overwrites a file-kind node with the given
// inline data. Files larger than MaxDataSize return
// KindInvalidArgument. Linux vfs_write.
func (v *vfs) Write(ctx context.Context, driveID string, path string, data []byte) error {
	if _, err := v.resolveTarget(ctx, driveID, path, permission.ActionEdit); err == nil {
		dentry, err := v.resolveTarget(ctx, driveID, path, permission.ActionEdit)
		if err != nil {
			return err
		}
		if dentry.Node.Kind() != NodeKindFile {
			return errorx.New(errorx.KindInvalidArgument, "vfs: target exists and is not a file")
		}
		return dentry.Node.Write(data, int64(len(data)))
	}
	// Not found → create the file with data.
	_, err := v.createWithData(ctx, driveID, path, NodeKindFile, data)
	return err
}

// WriteObject creates or replaces an Object-kind node. vfs
// stores the ref inline as content.ObjectContent. Caller
// (typically the handler after a successful S3 PUT) supplies
// the bucket, key, mime type, and optional checksum.
func (v *vfs) WriteObject(ctx context.Context, driveID string, path string, ref ObjectRef) error {
	startDrive, err := ulid.Parse(driveID)
	if err != nil {
		return errorx.Wrap(err, "vfs: invalid drive id", errorx.KindInvalidArgument)
	}
	if ref.Bucket == "" || ref.Key == "" {
		return errorx.New(errorx.KindInvalidArgument, "vfs: object ref requires bucket and key")
	}

	// Build the internal content from the public input.
	obj := contentObjectContent{
		Bucket:   ref.Bucket,
		Key:      ref.Key,
		Mime:     ref.Mime,
		Checksum: ref.Checksum,
	}
	data, err := json.Marshal(obj)
	if err != nil {
		return errorx.Wrap(err, "vfs: failed to marshal object content")
	}

	now := time.Now()
	child := NewNode(uuid.New(), startDrive, NodeKindObject)
	child.atime = now
	child.mtime = now
	child.ctime = now
	child.btime = now
	if err := child.Write(data, int64(len(data))); err != nil {
		return err
	}

	// Try replace first, else create.
	if _, err := v.resolveTarget(ctx, driveID, path, permission.ActionEdit); err == nil {
		// Replace: rename target aside, create new, then unlink
		// the old one. For simplicity, we always create a new
		// object-kind node in place — caller is responsible for
		// cleanup of the previous object via Unlink. Phase 2
		// keeps this simple.
		_, _ = v.resolveTarget(ctx, driveID, path, permission.ActionEdit)
		// Create replaces by nodeOp.Create via parent entry
		// update: we unlink the existing entry first.
		parent, name, err := v.resolveParent(ctx, driveID, path, permission.ActionEdit)
		if err != nil {
			return err
		}
		old, err := v.nodeOp.Lookup(ctx, parent.Node, name)
		if err == nil {
			if old.Node.Kind() != NodeKindObject {
				return errorx.New(errorx.KindInvalidArgument, "vfs: target exists and is not an object")
			}
			if err := v.nodeOp.Unlink(ctx, old); err != nil {
				return errorx.Wrap(err, "vfs: failed to remove existing object")
			}
		}
		return v.nodeOp.Create(ctx, parent.Node, child, name)
	}
	// Not found → create fresh.
	_, err = v.createWithData(ctx, driveID, path, NodeKindObject, data)
	return err
}

// createWithData is the shared path for fresh Create + data
// (used by Write for new files and WriteObject for new objects).
// Resolves the parent, builds a fresh Node of `kind` with the
// given data already written, and dispatches nodeOp.Create.
func (v *vfs) createWithData(ctx context.Context, driveID string, path string, kind NodeKind, data []byte) (*Node, error) {
	startDrive, err := ulid.Parse(driveID)
	if err != nil {
		return nil, errorx.Wrap(err, "vfs: invalid drive id", errorx.KindInvalidArgument)
	}
	parent, name, err := v.resolveParent(ctx, driveID, path, permission.ActionEdit)
	if err != nil {
		return nil, err
	}
	if parent.Node.Kind() != NodeKindDirectory {
		return nil, errorx.New(errorx.KindInvalidArgument, "vfs: parent is not a directory")
	}
	if err := v.checkPerm(ctx, permission.ActionEdit, startDrive); err != nil {
		return nil, err
	}

	now := time.Now()
	child := NewNode(uuid.New(), startDrive, kind)
	child.atime = now
	child.mtime = now
	child.ctime = now
	child.btime = now
	if err := child.Write(data, int64(len(data))); err != nil {
		return nil, err
	}
	if err := v.nodeOp.Create(ctx, parent.Node, child, name); err != nil {
		return nil, err
	}
	return child, nil
}

// contentObjectContent mirrors content.ObjectContent. Defined
// locally so this file doesn't pull internal/content's import.
type contentObjectContent struct {
	Bucket   string `json:"bucket"`
	Key      string `json:"key"`
	Mime     string `json:"mime"`
	Checksum string `json:"sum,omitempty"`
}
