package vfs

import (
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/mandacode-labs/mdrive/internal/fs"
	"github.com/mandacode-labs/mdrive/internal/fs/s3"
)

// vfs is the concrete implementation of fs.VFS.
type vfs struct {
	nodeOp    fs.NodeOperation
	superop   fs.SuperOperation
	storageOp fs.StorageOperation
	presigner s3.Presigner // default IRSA
}

// Config groups the dependencies of vfs.
type Config struct {
	NodeOp    fs.NodeOperation
	SuperOp   fs.SuperOperation
	StorageOp fs.StorageOperation
	DefaultS3 *awss3.Client // IRSA fallback
}

// New constructs an fs.VFS implementation.
func New(cfg Config) fs.VFS {
	return &vfs{
		nodeOp:    cfg.NodeOp,
		superop:   cfg.SuperOp,
		storageOp: cfg.StorageOp,
		presigner: s3.NewDefaultPresigner(cfg.DefaultS3),
	}
}