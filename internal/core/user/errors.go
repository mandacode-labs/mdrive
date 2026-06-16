package user

import "errors"

// User-domain sentinel errors. Use errors.Is(err, user.ErrXxx) to check.
var (
	// ErrNotFound is returned when a user is not present in the repository.
	ErrNotFound = errors.New("user: not found")

	// ErrProviderRequired is returned when the OIDC provider is missing.
	ErrProviderRequired = errors.New("user: provider is required")

	// ErrProviderIDRequired is returned when the OIDC provider_id is missing.
	ErrProviderIDRequired = errors.New("user: provider_id is required")

	// ErrNameRequired is returned when the display name is missing.
	ErrNameRequired = errors.New("user: name is required")
)
