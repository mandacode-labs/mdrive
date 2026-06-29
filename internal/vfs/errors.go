package vfs

import "github.com/mandacode-labs/mdrive/internal/errorx"

var (
	ErrNotFound     = &errorx.Error{Kind: errorx.KindNotFound, Msg: "vfs: not found"}
	ErrNotDirectory = &errorx.Error{Kind: errorx.KindBadRequest, Msg: "vfs: not a directory"}
	ErrInvalidPath  = &errorx.Error{Kind: errorx.KindBadRequest, Msg: "vfs: invalid path"}
	ErrCrossDrive   = &errorx.Error{Kind: errorx.KindBadRequest, Msg: "vfs: cross-drive move not supported"}
	ErrMountCycle   = &errorx.Error{Kind: errorx.KindBadRequest, Msg: "vfs: mount cycle detected"}
	ErrPathTooDeep  = &errorx.Error{Kind: errorx.KindBadRequest, Msg: "vfs: max mount hops exceeded"}
)
