package user

import "github.com/mandacode-labs/mdrive/internal/errorx"

var (
	ErrNotFound          = errorx.New(errorx.KindNotFound, "user: not found")
	ErrProviderRequired  = errorx.New(errorx.KindBadRequest, "user: provider is required")
	ErrProviderIDRequired = errorx.New(errorx.KindBadRequest, "user: provider_id is required")
	ErrNameRequired      = errorx.New(errorx.KindBadRequest, "user: name is required")
)
