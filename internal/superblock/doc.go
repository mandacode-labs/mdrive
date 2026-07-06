// Package superblock models the filesystem root inode carrier
// for each drive. Linux's struct super_block holds the mount
// info, filesystem type, and the root dentry; we mirror that
// 1:1 with a dedicated table and a thin operation surface.
//
// vfs uses this package to start path resolution: every Walk
// needs the root inode of the drive it's starting from, and
// that's exactly what GetRootNodeID returns.
package superblock