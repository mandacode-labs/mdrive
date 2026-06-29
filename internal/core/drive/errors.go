package drive

import "github.com/mandacode-labs/mdrive/internal/errorx"

var (
	ErrNotFound          = &errorx.Error{Kind: errorx.KindNotFound, Msg: "drive: not found"}
	ErrInvalidName       = &errorx.Error{Kind: errorx.KindBadRequest, Msg: "drive: invalid name"}
	ErrInvalidBucket     = &errorx.Error{Kind: errorx.KindBadRequest, Msg: "drive: storage bucket is required"}
	ErrInvalidRegion     = &errorx.Error{Kind: errorx.KindBadRequest, Msg: "drive: storage region is required"}
	ErrInvalidCredentials = &errorx.Error{Kind: errorx.KindBadRequest, Msg: "drive: storage credentials are required"}
	ErrDecryptionFailed  = &errorx.Error{Kind: errorx.KindBadRequest, Msg: "drive: failed to decrypt storage credentials"}
)
