package vfs

import (
	"github.com/mandacode-labs/mdrive/internal/fs"
)

// vfs is the concrete implementation of fs.VFS.
type vfs struct {
	nodeOp    fs.NodeOperation
	superop   fs.SuperOperation
	storageOp fs.StorageOperation
	// defaultStorage is the app-level default Storage (set
	// at app startup). nil = SDK default chain (IRSA).
	defaultStorage *fs.Storage
}

// Config groups the dependencies of vfs.
type Config struct {
	NodeOp         fs.NodeOperation
	SuperOp        fs.SuperOperation
	StorageOp      fs.StorageOperation
	DefaultStorage *fs.Storage // nil = SDK default (IRSA)
}

// New constructs an fs.VFS implementation.
func New(cfg Config) fs.VFS {
	return &vfs{
		nodeOp:          cfg.NodeOp,
		superop:         cfg.SuperOp,
		storageOp:       cfg.StorageOp,
		defaultStorage: cfg.DefaultStorage,
	}
}