package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
)

// SetTimes — Linux inode_setattr. Caller (Service.SetTimes)
// is expected to have already set in-memory fields via
// node.SetTimes. We just persist.
func (v *vfs) SetTimes(ctx context.Context, dentry *fs.Dentry) error {
	if dentry == nil || dentry.Node == nil {
		return errorx.New(errorx.KindInvalidArgument, "fs: SetTimes requires a dentry")
	}
	if err := v.nodeOp.Persist(ctx, dentry.Node); err != nil {
		return err
	}
	return nil
}
