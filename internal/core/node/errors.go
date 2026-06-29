package node

import "github.com/mandacode-labs/mdrive/internal/errorx"

type Error struct {
	kind errorx.Kind
	Msg  string
}

func (e *Error) Error() string { return e.Msg }
func (e *Error) Kind() errorx.Kind { return e.kind }

var (
	ErrNotFound          = &Error{kind: errorx.NotFound, Msg: "node: not found"}
	ErrEntryExists       = &Error{kind: errorx.Conflict, Msg: "node: entry already exists"}
	ErrEntryNotFound     = &Error{kind: errorx.NotFound, Msg: "node: entry not found"}
	ErrNotDirectory      = &Error{kind: errorx.BadRequest, Msg: "node: not a directory"}
	ErrInvalidType       = &Error{kind: errorx.BadRequest, Msg: "node: invalid type for operation"}
	ErrInvalidName       = &Error{kind: errorx.BadRequest, Msg: "node: invalid name"}
	ErrInvalidReference  = &Error{kind: errorx.BadRequest, Msg: "node: invalid object reference"}
	ErrInvalidSize       = &Error{kind: errorx.BadRequest, Msg: "node: invalid size"}
	ErrNoContent         = &Error{kind: errorx.NotFound, Msg: "node: no content"}
	ErrContentTooLarge   = &Error{kind: errorx.BadRequest, Msg: "node: content exceeds maximum size"}
	ErrRevisionConflict  = &Error{kind: errorx.Conflict, Msg: "node: revision conflict"}
	ErrIsDirectory       = &Error{kind: errorx.BadRequest, Msg: "node: target is a directory"}
	ErrIsObject          = &Error{kind: errorx.BadRequest, Msg: "node: target is an S3 object; use the presign-download endpoint"}
	ErrInvalidMoveOverwrite = &Error{kind: errorx.BadRequest, Msg: "node: cannot overwrite entry of different type"}
	ErrSymlinkCycle      = &Error{kind: errorx.BadRequest, Msg: "node: symlink cycle or too many hops"}
)
