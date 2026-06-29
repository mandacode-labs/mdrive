package vfs

import "github.com/mandacode-labs/mdrive/internal/errorx"

type Error struct {
	kind errorx.Kind
	Msg  string
}

func (e *Error) Error() string { return e.Msg }
func (e *Error) Kind() errorx.Kind { return e.kind }

var (
	ErrNotFound     = &Error{kind: errorx.NotFound, Msg: "vfs: not found"}
	ErrNotDirectory = &Error{kind: errorx.BadRequest, Msg: "vfs: not a directory"}
	ErrInvalidPath  = &Error{kind: errorx.BadRequest, Msg: "vfs: invalid path"}
	ErrCrossDrive   = &Error{kind: errorx.BadRequest, Msg: "vfs: cross-drive move not supported"}
	ErrMountCycle   = &Error{kind: errorx.BadRequest, Msg: "vfs: mount cycle detected"}
	ErrPathTooDeep  = &Error{kind: errorx.BadRequest, Msg: "vfs: max mount hops exceeded"}
)
