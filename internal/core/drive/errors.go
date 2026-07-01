package drive

import "github.com/mandacode-labs/mdrive/internal/errorx"

var (
	ErrNotFound          = errorx.New(errorx.KindNotFound, "drive: not found")
	ErrInvalidName       = errorx.New(errorx.KindBadRequest, "drive: invalid name")
	ErrInvalidBucket     = errorx.New(errorx.KindBadRequest, "drive: storage bucket is required")
	ErrInvalidRegion     = errorx.New(errorx.KindBadRequest, "drive: storage region is required")
	ErrInvalidCredentials = errorx.New(errorx.KindBadRequest, "drive: storage credentials are required")
	ErrDecryptionFailed  = errorx.New(errorx.KindBadRequest, "drive: failed to decrypt storage credentials")
	ErrOwnerNotFound     = errorx.New(errorx.KindForbidden, "drive: owner not found")
)
