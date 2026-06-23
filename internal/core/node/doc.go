// Package node is the inode abstraction of mdrive.
//
// A Node holds metadata (id, type, size, timestamps, flags) and,
// for small items, inline content up to MaxContentSize. The node
// is intentionally ignorant of its drive, parent, or name — those
// live in the drive (root_node_id) and parent directory
// (DirContent), exactly as in Unix where i_parent and i_name are
// absent from the inode.
//
// S3 data state is NOT tracked here. For object nodes, the actual
// S3 data existence is checked lazily on read (HEAD to S3).
//
// Package hierarchy:
//
//	node  -- low-level inode + persistence
//	vfs   -- path resolution, permissions, S3 I/O; consumes node.Service
package node
