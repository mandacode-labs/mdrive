package drive

import (
	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

func parseDriveID(id string) (ulid.ULID, error) {
	u, err := ulid.Parse(id)
	if err != nil {
		return ulid.ULID{}, errorx.Wrap(err, "drive: invalid drive id", errorx.KindInvalidArgument)
	}
	return u, nil
}

func errorxKindInvalidArgument(msg string) error {
	return errorx.New(errorx.KindInvalidArgument, msg)
}

func errorxKindNotFound(msg string) error {
	return errorx.New(errorx.KindNotFound, msg)
}

func errorxKindFailedPrecondition(msg string) error {
	return errorx.New(errorx.KindFailedPrecondition, msg)
}