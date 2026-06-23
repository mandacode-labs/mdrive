package upload

import "errors"

// Common errors returned by the upload package.
var (
	// Registry errors.
	ErrNotFound    = errors.New("upload: token not found")
	ErrTokenExists = errors.New("upload: token already exists")

	// Orchestration errors returned by Service.
	ErrUploadMismatch    = errors.New("upload: token does not match drive")
	ErrObjectNotUploaded = errors.New("upload: S3 object was not uploaded")
	ErrPermission        = errors.New("upload: permission denied")
)
