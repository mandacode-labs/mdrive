package node

import "github.com/mandacode-labs/mdrive/internal/errorx"

var (
	ErrNotFound          = errorx.New(errorx.KindNotFound, "node: not found")
	ErrEntryExists       = errorx.New(errorx.KindConflict, "node: entry already exists")
	ErrEntryNotFound     = errorx.New(errorx.KindNotFound, "node: entry not found")
	ErrNotDirectory      = errorx.New(errorx.KindBadRequest, "node: not a directory")
	ErrInvalidType       = errorx.New(errorx.KindBadRequest, "node: invalid type for operation")
	ErrInvalidName       = errorx.New(errorx.KindBadRequest, "node: invalid name")
	ErrInvalidReference  = errorx.New(errorx.KindBadRequest, "node: invalid object reference")
	ErrInvalidSize       = errorx.New(errorx.KindBadRequest, "node: invalid size")
	ErrNoContent         = errorx.New(errorx.KindNotFound, "node: no content")
	ErrContentTooLarge   = errorx.New(errorx.KindBadRequest, "node: content exceeds maximum size")
	ErrRevisionConflict  = errorx.New(errorx.KindConflict, "node: revision conflict")
	ErrIsDirectory       = errorx.New(errorx.KindBadRequest, "node: target is a directory")
	ErrIsObject          = errorx.New(errorx.KindBadRequest, "node: target is an S3 object; use the presign-download endpoint")
	ErrInvalidMoveOverwrite = errorx.New(errorx.KindBadRequest, "node: cannot overwrite entry of different type")
	ErrSymlinkCycle      = errorx.New(errorx.KindBadRequest, "node: symlink cycle or too many hops")
)
