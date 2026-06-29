package upload

import "github.com/mandacode-labs/mdrive/internal/errorx"

type Error struct {
	kind errorx.Kind
	Msg  string
}

func (e *Error) Error() string { return e.Msg }
func (e *Error) Kind() errorx.Kind { return e.kind }

var (
	ErrNotFound                = &Error{kind: errorx.BadRequest, Msg: "upload: token not found"}
	ErrUploadMismatch          = &Error{kind: errorx.BadRequest, Msg: "upload: token does not match drive"}
	ErrUploadOwnershipMismatch = &Error{kind: errorx.Forbidden, Msg: "upload: token does not match user"}
	ErrObjectNotUploaded       = &Error{kind: errorx.NotFound, Msg: "upload: S3 object was not uploaded"}
)
