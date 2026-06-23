package drive

import "errors"

// ErrPermission is returned when a permission check fails.
var ErrPermission = errors.New("drive: permission denied")
