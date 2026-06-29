package node

import "github.com/mandacode-labs/mdrive/internal/errorx"

var (
	ErrNotFound          = &errorx.Error{Kind: errorx.KindNotFound, Msg: "node: not found"}
	ErrEntryExists       = &errorx.Error{Kind: errorx.KindConflict, Msg: "node: entry already exists"}
	ErrEntryNotFound     = &errorx.Error{Kind: errorx.KindNotFound, Msg: "node: entry not found"}
	ErrNotDirectory      = &errorx.Error{Kind: errorx.KindBadRequest, Msg: "node: not a directory"}
	ErrInvalidType       = &errorx.Error{Kind: errorx.KindBadRequest, Msg: "node: invalid type for operation"}
	ErrInvalidName       = &errorx.Error{Kind: errorx.KindBadRequest, Msg: "node: invalid name"}
	ErrInvalidReference  = &errorx.Error{Kind: errorx.KindBadRequest, Msg: "node: invalid object reference"}
	ErrInvalidSize       = &errorx.Error{Kind: errorx.KindBadRequest, Msg: "node: invalid size"}
	ErrNoContent         = &errorx.Error{Kind: errorx.KindNotFound, Msg: "node: no content"}
	ErrContentTooLarge   = &errorx.Error{Kind: errorx.KindBadRequest, Msg: "node: content exceeds maximum size"}
	ErrRevisionConflict  = &errorx.Error{Kind: errorx.KindConflict, Msg: "node: revision conflict"}
	ErrIsDirectory       = &errorx.Error{Kind: errorx.KindBadRequest, Msg: "node: target is a directory"}
	ErrIsObject          = &errorx.Error{Kind: errorx.KindBadRequest, Msg: "node: target is an S3 object; use the presign-download endpoint"}
	ErrInvalidMoveOverwrite = &errorx.Error{Kind: errorx.KindBadRequest, Msg: "node: cannot overwrite entry of different type"}
	ErrSymlinkCycle      = &errorx.Error{Kind: errorx.KindBadRequest, Msg: "node: symlink cycle or too many hops"}
)
