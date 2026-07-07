// Package vfs is the inode layer of the fs subsystem. It
// mirrors Linux's vfs_* functions; fs.Service handles path
// lookup and permission checks. fs.New returns the fs.VFS
// interface; vvs.New constructs it.
package vfs
