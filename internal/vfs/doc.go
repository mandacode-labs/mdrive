// Package vfs is mdrive's inode + superblock layer. It models the
// canonical virtual filesystem operations (path → walk → inode
// mutations, mount crossing, symlink follow) and delegates
// persistence to the ent-backed nodeop and superop impls.
package vfs
