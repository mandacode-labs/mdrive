// Package drive is the drive lifecycle domain. A drive is a
// multi-tenant storage unit: it has a name, an owner, an
// optional description, and a soft-delete state. The
// filesystem root inode lives in the superblock package; this
// package owns only the drive's own metadata.
//
// Handlers call into this package to create / list / soft-delete
// / restore / purge drives. vfs uses superblock.Operation to
// reach a drive's root inode when resolving a path.
package drive