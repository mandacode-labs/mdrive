package drive

import "github.com/mandacode-labs/mdrive/internal/errorx"

type Error struct {
	kind errorx.Kind
	Msg  string
}

func (e *Error) Error() string { return e.Msg }
func (e *Error) Kind() errorx.Kind { return e.kind }

var (
	ErrNotFound          = &Error{kind: errorx.NotFound, Msg: "drive: not found"}
	ErrInvalidName       = &Error{kind: errorx.BadRequest, Msg: "drive: invalid name"}
	ErrInvalidBucket     = &Error{kind: errorx.BadRequest, Msg: "drive: storage bucket is required"}
	ErrInvalidRegion     = &Error{kind: errorx.BadRequest, Msg: "drive: storage region is required"}
	ErrInvalidCredentials = &Error{kind: errorx.BadRequest, Msg: "drive: storage credentials are required"}
	ErrDecryptionFailed  = &Error{kind: errorx.BadRequest, Msg: "drive: failed to decrypt storage credentials"}
)
