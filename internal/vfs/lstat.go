package vfs

import "context"

// Lstat returns the dentry's node without following a trailing
// symlink. Linux lstat(2).
func (v *vfs) Lstat(ctx context.Context, dentry *Dentry) (*Node, error) {
	if dentry == nil {
		return nil, nil
	}
	return dentry.Node, nil
}
