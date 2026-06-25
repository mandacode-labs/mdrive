package upload

import "errors"

// Common errors returned by the upload package.
//
// Permission errors come from the handler layer (via
// permission.ErrPermission) and are not re-defined here.
var (
	// TokenRegistry errors.
	ErrNotFound = errors.New("upload: token not found")

	// Orchestration errors returned by Service.
	ErrUploadMismatch         = errors.New("upload: token does not match drive")
	ErrUploadOwnershipMismatch = errors.New("upload: token does not match user")
	ErrObjectNotUploaded      = errors.New("upload: S3 object was not uploaded")
)
