package vfs

import (
	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// parseDriveID parses a string drive id into ulid.ULID. Returns
// KindInvalidArgument on failure.
func parseDriveID(id string) (ulid.ULID, error) {
	u, err := ulid.Parse(id)
	if err != nil {
		return ulid.ULID{}, errorx.Wrap(err, "vfs: invalid drive id", errorx.KindInvalidArgument)
	}
	return u, nil
}
