package upload

import "github.com/mandacode-labs/mdrive/internal/errorx"

var (
	ErrNotFound                = &errorx.Error{Kind: errorx.KindBadRequest, Msg: "upload: token not found"}
	ErrUploadMismatch          = &errorx.Error{Kind: errorx.KindBadRequest, Msg: "upload: token does not match drive"}
	ErrUploadOwnershipMismatch = &errorx.Error{Kind: errorx.KindForbidden, Msg: "upload: token does not match user"}
	ErrObjectNotUploaded       = &errorx.Error{Kind: errorx.KindNotFound, Msg: "upload: S3 object was not uploaded"}
)
