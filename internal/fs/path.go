package fs

import (
	"context"
	"strings"

	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// doPathParent resolves the parent of path and returns
// (parent *Dentry, leafName). Mirrors filename_lookup +
// path_parentat.
func (f *fs) doPathParent(ctx context.Context, driveID, path string) (*Dentry, string, error) {
	driveULID, err := ulid.Parse(driveID)
	if err != nil {
		return nil, "", errorx.New(errorx.KindInvalidArgument, "fs: invalid drive id")
	}
	parentPath, name := splitParent(path)
	if parentPath == "" {
		parentPath = "/"
	}
	parent, err := f.vfs.Walk(ctx, driveULID, parentPath, true)
	if err != nil {
		return nil, "", err
	}
	if err := f.requireRead(ctx, parent.DriveID); err != nil {
		return nil, "", err
	}
	return parent, name, nil
}

// splitParent splits an absolute path into (parent_path, leaf).
func splitParent(p string) (parent, name string) {
	if p == "" {
		return "", ""
	}
	i := strings.LastIndex(p, "/")
	if i < 0 {
		return ".", p
	}
	if i == 0 {
		return "/", p[1:]
	}
	return p[:i], p[i+1:]
}

// walkResolve resolves a path into a Dentry with follow=true.
func (f *fs) walkResolve(ctx context.Context, driveID, path string) (*Dentry, error) {
	id, err := ulid.Parse(driveID)
	if err != nil {
		return nil, errorx.New(errorx.KindInvalidArgument, "fs: invalid drive id")
	}
	return f.vfs.Walk(ctx, id, path, true)
}

// walkForKind resolves a path and checks the leaf is of the
// expected kind.
func (f *fs) walkForKind(ctx context.Context, driveID, path string, kind NodeKind) (*Dentry, error) {
	dentry, err := f.walkResolve(ctx, driveID, path)
	if err != nil {
		return nil, err
	}
	if dentry.Node.Kind() != kind {
		return nil, errorx.New(errorx.KindInvalidArgument, "fs: wrong node kind")
	}
	return dentry, nil
}
