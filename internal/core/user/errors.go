package user

import "github.com/mandacode-labs/mdrive/internal/errorx"

type Error struct {
	kind errorx.Kind
	Msg  string
}

func (e *Error) Error() string { return e.Msg }
func (e *Error) Kind() errorx.Kind { return e.kind }

var (
	ErrNotFound          = &Error{kind: errorx.NotFound, Msg: "user: not found"}
	ErrProviderRequired  = &Error{kind: errorx.BadRequest, Msg: "user: provider is required"}
	ErrProviderIDRequired = &Error{kind: errorx.BadRequest, Msg: "user: provider_id is required"}
	ErrNameRequired      = &Error{kind: errorx.BadRequest, Msg: "user: name is required"}
)
