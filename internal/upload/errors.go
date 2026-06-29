package upload

import "github.com/mandacode-labs/mdrive/internal/errorx"

var (
	ErrNotFound                = errorx.New(errorx.KindBadRequest, "upload: token not found")
	ErrUploadMismatch          = errorx.New(errorx.KindBadRequest, "upload: token does not match drive")
	ErrUploadOwnershipMismatch = errorx.New(errorx.KindForbidden, "upload: token does not match user")
	ErrObjectNotUploaded       = errorx.New(errorx.KindNotFound, "upload: S3 object was not uploaded")
)
