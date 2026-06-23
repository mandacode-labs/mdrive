package vfs

import "errors"

var (
	ErrNotFound          = errors.New("vfs: not found")
	ErrNotDirectory      = errors.New("vfs: not a directory")
	ErrPermission        = errors.New("vfs: permission denied")
	ErrInvalidPath       = errors.New("vfs: invalid path")
	ErrCrossDrive        = errors.New("vfs: cross-drive move not supported")
	ErrUploadMismatch    = errors.New("vfs: upload token does not match drive")
	ErrObjectNotUploaded = errors.New("vfs: S3 object was not uploaded")
	ErrMountCycle        = errors.New("vfs: mount cycle detected")
	ErrPathTooDeep       = errors.New("vfs: max mount hops exceeded")
)
