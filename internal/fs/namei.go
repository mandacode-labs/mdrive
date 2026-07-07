package fs

import (
	"context"

	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// Walk resolves a path into a Dentry. Mirrors link_path_walk.
func (f *fs) Walk(ctx context.Context, driveID, path string) (*Dentry, error) {
	id, err := ulid.Parse(driveID)
	if err != nil {
		return nil, errorx.New(errorx.KindInvalidArgument, "fs: invalid drive id")
	}
	return f.vfs.lookup(ctx, id, path, true)
}

// WalkOne resolves a single component under `parent`.
// Mirrors lookup_one. Permission is the caller's job.
func (f *fs) WalkOne(ctx context.Context, parent *Dentry, name string) (*Dentry, error) {
	return f.vfs.walkOne(ctx, parent, name)
}
