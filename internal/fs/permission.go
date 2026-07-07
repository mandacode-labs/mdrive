package fs

import (
	"context"
	"strings"

	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/auth"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs/permission"
)

// userID extracts the caller's user id from the request ctx.
func (f *fs) userID(ctx context.Context) string {
	return auth.UserIDFromContext(ctx)
}

// checkPerm gates a drive-level permission.
func (f *fs) checkPerm(ctx context.Context, action permission.Action, driveID ulid.ULID) error {
	uid := f.userID(ctx)
	ok, err := f.perm.Check(ctx, uid, action, permission.ObjectTypeDrive, driveID.String())
	if err != nil {
		return errorx.Wrap(err, "fs: permission check failed", errorx.KindUnavailable)
	}
	if !ok {
		return errorx.New(errorx.KindPermissionDenied, "fs: permission denied")
	}
	return nil
}

// requireView gates the read side.
func (f *fs) requireView(ctx context.Context, driveID ulid.ULID) error {
	return f.checkPerm(ctx, permission.ActionView, driveID)
}

// requireEdit gates the write side.
func (f *fs) requireEdit(ctx context.Context, driveID ulid.ULID) error {
	return f.checkPerm(ctx, permission.ActionEdit, driveID)
}

// doPathParent resolves the parent of `path` and returns
// (parent *Dentry, leafName, error). Mirrors filename_lookup
// + path_parentat. Mutating ops add ActionEdit on top.
func (f *fs) doPathParent(ctx context.Context, driveID, path string) (*Dentry, string, error) {
	driveULID, err := ulid.Parse(driveID)
	if err != nil {
		return nil, "", errorx.New(errorx.KindInvalidArgument, "fs: invalid drive id")
	}
	parentPath, name := splitParent(path)
	if parentPath == "" {
		parentPath = "/"
	}
	parent, err := f.vfs.lookup(ctx, driveULID, parentPath, true)
	if err != nil {
		return nil, "", err
	}
	if err := f.requireView(ctx, parent.DriveID); err != nil {
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
