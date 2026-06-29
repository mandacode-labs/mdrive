package user

import "github.com/mandacode-labs/mdrive/internal/errorx"

var (
	ErrNotFound          = &errorx.Error{Kind: errorx.KindNotFound, Msg: "user: not found"}
	ErrProviderRequired  = &errorx.Error{Kind: errorx.KindBadRequest, Msg: "user: provider is required"}
	ErrProviderIDRequired = &errorx.Error{Kind: errorx.KindBadRequest, Msg: "user: provider_id is required"}
	ErrNameRequired      = &errorx.Error{Kind: errorx.KindBadRequest, Msg: "user: name is required"}
)
