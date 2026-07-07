package fs

import (
	"context"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs/content"
)

// Create inserts a new inode of the given kind. Mirrors
// creat(2) + mkdir(2) + openat(O_CREAT) + mknod(2).
// ActionEdit on the parent drive.
func (f *fs) Create(ctx context.Context, driveID, path string, kind NodeKind) (Stat, error) {
	return f.doCreate(ctx, driveID, path, kind, ObjectRef{}, 0)
}

// doCreate is shared by Create and CreateObject. Object kind
// carries S3 metadata; other kinds ignore ref/size.
func (f *fs) doCreate(ctx context.Context, driveID, path string, kind NodeKind, ref ObjectRef, size int64) (Stat, error) {
	parent, name, err := f.doPathParent(ctx, driveID, path)
	if err != nil {
		return Stat{}, err
	}
	if err := f.requireEdit(ctx, parent.DriveID); err != nil {
		return Stat{}, err
	}

	if kind == NodeKindDirectory {
		node, err := f.vfs.Mkdir(ctx, parent, name)
		if err != nil {
			return Stat{}, err
		}
		return NodeToStat(node), nil
	}

	child := NewNode(uuid.New(), parent.Node.SuperblockID(), kind)
	if kind == NodeKindObject {
		oc := &content.ObjectContent{
			Bucket:   ref.Bucket,
			Key:      ref.Key,
			Mime:     ref.Mime,
			Checksum: ref.Checksum,
		}
		data, err := oc.Marshal()
		if err != nil {
			return Stat{}, errorx.Wrap(err, "fs: failed to marshal object content", errorx.KindInternal)
		}
		if err := child.Write(data, size); err != nil {
			return Stat{}, err
		}
	}
	if err := f.vfs.Create(ctx, parent, child, name); err != nil {
		return Stat{}, err
	}
	return NodeToStat(child), nil
}
