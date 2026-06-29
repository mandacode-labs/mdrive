package vfs

import "github.com/mandacode-labs/mdrive/internal/errorx"

var (
	ErrNotFound     = errorx.New(errorx.KindNotFound, "vfs: not found")
	ErrNotDirectory = errorx.New(errorx.KindBadRequest, "vfs: not a directory")
	ErrInvalidPath  = errorx.New(errorx.KindBadRequest, "vfs: invalid path")
	ErrCrossDrive   = errorx.New(errorx.KindBadRequest, "vfs: cross-drive move not supported")
	ErrMountCycle   = errorx.New(errorx.KindBadRequest, "vfs: mount cycle detected")
	ErrPathTooDeep  = errorx.New(errorx.KindBadRequest, "vfs: max mount hops exceeded")
)
