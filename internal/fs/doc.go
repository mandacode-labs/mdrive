// Package fs is mdrive's filesystem layer.
//
// Layering mirrors Linux fs/:
//
//	SYSCALL_DEFINE*(name) → Service method
//	do_*(...)             → fs.doX
//	vfs_*(...)            → fs.vfs.X (vfs subpackage)
//
// Permission checks live on Service; the vfs subpackage
// operates on already-resolved *Dentry.
package fs
