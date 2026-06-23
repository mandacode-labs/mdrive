package vfs

import "errors"

var (
	ErrNotFound     = errors.New("vfs: not found")
	ErrNotDirectory = errors.New("vfs: not a directory")
	ErrInvalidPath  = errors.New("vfs: invalid path")
	ErrCrossDrive   = errors.New("vfs: cross-drive move not supported")
	ErrMountCycle   = errors.New("vfs: mount cycle detected")
	ErrPathTooDeep  = errors.New("vfs: max mount hops exceeded")
)
