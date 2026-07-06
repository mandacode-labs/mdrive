package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/auth"
)

// isAdmin is the admin check used by vfs high-level wrappers.
// The actual auth model (token claim, header, etc.) is owned by
// the auth package; vfs just queries it.
func (v *vfs) isAdmin(ctx context.Context) bool {
	return auth.IsAdmin(ctx)
}
