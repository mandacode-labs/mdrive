// Package superop is the per-drive root inode lookup +
// data-access layer, ent-backed.
//
// Operation is the small lookup surface vfs needs (mirrors
// Linux super_operations). Repository is the broader CRUD used
// by drive lifecycle (create / soft-delete / purge) to keep the
// superblock in step with the drive row. Mirrors nodeop's split.
package superop
