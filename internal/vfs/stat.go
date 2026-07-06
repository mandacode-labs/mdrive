package vfs

import (
	"context"
)

// Stat returns the dentry's node. Linux stat(2) — caller is
// expected to have followed trailing symlinks (Walk with
// follow=true) before calling.
func (v *vfs) Stat(ctx context.Context, dentry *Dentry) (*Node, error) {
	if dentry == nil {
		return nil, nil
	}
	return dentry.Node, nil
}
