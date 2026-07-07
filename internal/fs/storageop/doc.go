// Package storageop is the ent-backed implementation of
// fs.StorageOperation. Lookups are keyed by superblock_id
// (not drive_id); the fs.Storage domain is separate from
// drive.Storage even though they share an ent table.
package storageop