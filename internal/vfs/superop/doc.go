// Package superop holds the superblock data model and its
// data-access contract.
//
// Linux analogy: every mounted filesystem has one
// `struct super_block` and a `struct super_operations` table.
// We expose the same shape here — Operation is the small
// lookup surface vfs needs to resolve a drive's root inode,
// and Repository is the broader CRUD used by drive lifecycle
// (create / soft-delete / purge) to keep the superblock in
// step with the drive row.
//
// Mirrors internal/vfs/nodeop's split: Operation is the
// interface vfs depends on, Repository is what callers that
// own the tx boundary (drive.Service) call directly.
package superop