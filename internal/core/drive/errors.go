package drive

import "errors"

// Drive-domain sentinel errors. Use errors.Is(err, drive.ErrXxx) to check.
var (
	// ErrNotFound is returned when a drive is not present in the repository.
	ErrNotFound = errors.New("drive: not found")

	// ErrInvalidName is returned when a drive name is empty or otherwise invalid.
	ErrInvalidName = errors.New("drive: invalid name")

	// ErrInvalidBucket is returned when storage bucket is missing.
	ErrInvalidBucket = errors.New("drive: storage bucket is required")

	// ErrInvalidRegion is returned when storage region is missing.
	ErrInvalidRegion = errors.New("drive: storage region is required")

	// ErrInvalidCredentials is returned when storage credentials are missing.
	ErrInvalidCredentials = errors.New("drive: storage credentials are required")

	// ErrDecryptionFailed is returned when stored credentials cannot be decrypted.
	// This usually means the master key changed or the data was created before
	// encryption was enabled; the owner must re-enter credentials.
	ErrDecryptionFailed = errors.New("drive: failed to decrypt storage credentials")
)
