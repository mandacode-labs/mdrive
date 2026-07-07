// Package fs is mdrive's filesystem layer.
//
// Layering mirrors Linux fs/:
//
//	SYSCALL_DEFINE*(name)  → Service method  (internal/fs/<kind>.go)
//	vfs_*(...)             → fs.VFS          (internal/fs/vfs/)
//	inode_operations(...)  → fs.NodeOperation (internal/fs/nodeop/)
//	super_operations(...)  → fs.SuperOperation (internal/fs/superop/)
//
// Service is the syscall surface (path-based, typed content
// in/out). The vfs subpackage operates on already-resolved
// *Dentry and never appears in Service signatures.
//
// Permission checks live in internal/fs/perm.go and use the
// perm.Service interface from internal/perm.
// Path resolution helpers live in internal/fs/path.go.
package fs