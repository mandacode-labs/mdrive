package fs

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// Stat is the user-visible inode metadata. Mirrors POSIX
// struct stat.
type Stat struct {
	InodeID  uuid.UUID
	Kind     NodeKind
	Size     int64
	NLink    uint32
	ATime    time.Time
	MTime    time.Time
	CTime    time.Time
	BTime    time.Time
	Flags    Flags
	Revision Revision
}

// NodeToStat converts a Node into the public Stat DTO.
func NodeToStat(n *Node) Stat {
	if n == nil {
		return Stat{}
	}
	return Stat{
		InodeID:  n.ID(),
		Kind:     n.Kind(),
		Size:     n.Size(),
		NLink:    n.NLink(),
		ATime:    n.ATime(),
		MTime:    n.MTime(),
		CTime:    n.CTime(),
		BTime:    n.BTime(),
		Flags:    n.Flags(),
		Revision: n.Revision(),
	}
}

// Stat returns inode metadata. Mirrors newfstatat(2) with
// follow (true → stat, false → lstat).
func (f *fs) Stat(ctx context.Context, driveID, path string, follow bool) (Stat, error) {
	id, err := ulid.Parse(driveID)
	if err != nil {
		return Stat{}, errorx.New(errorx.KindInvalidArgument, "fs: invalid drive id")
	}
	dentry, err := f.vfs.Walk(ctx, id, path, follow)
	if err != nil {
		return Stat{}, err
	}
	if err := f.requireView(ctx, dentry.DriveID); err != nil {
		return Stat{}, err
	}
	return f.vfs.Getattr(ctx, dentry)
}

// SetTimes updates atime/mtime on the resolved node.
// Mirrors utimensat(2). ActionEdit on the resolved drive.
// Ctime is bumped by the storage layer; not user-settable.
func (f *fs) SetTimes(ctx context.Context, driveID, path string, atime, mtime time.Time) error {
	id, err := ulid.Parse(driveID)
	if err != nil {
		return errorx.New(errorx.KindInvalidArgument, "fs: invalid drive id")
	}
	dentry, err := f.vfs.Walk(ctx, id, path, true)
	if err != nil {
		return err
	}
	if err := f.requireEdit(ctx, dentry.DriveID); err != nil {
		return err
	}
	dentry.Node.SetTimes(atime, mtime, time.Now(), dentry.Node.BTime())
	return f.vfs.SetTimes(ctx, dentry)
}
