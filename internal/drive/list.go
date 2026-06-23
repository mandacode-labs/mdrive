package drive

import (
	"context"
	"time"

	coredrive "github.com/mandacode-labs/mdrive/internal/core/drive"
)

// ListByOwner returns all active drives owned by ownerID. The
// caller (typically the handler) is responsible for ensuring
// the requesting user is ownerID; a user can only list their
// own drives. The service does not re-check identity because it
// has no session-layer concept of "the requesting user" — the
// handler is the security boundary. Misuse (passing another
// user's ID) leaks that user's drive listing, so callers must
// enforce.
//
// The method intentionally does not take a separate
// requestingUserID parameter: passing both ownerID and a
// requestingUserID and checking they match would be defensive
// against the footgun, but the check would be unenforceable
// without session context. Document the contract instead.
func (s *Service) ListByOwner(ctx context.Context, ownerID string) ([]*coredrive.Drive, error) {
	return s.Drive.ListByOwner(ctx, ownerID)
}

// ListDeleted returns soft-deleted drives. Admin-only: the caller
// must pass isAdmin=true. The handler is the source of truth for
// "is this user an admin" (via the auth layer); the service
// re-checks here so a non-admin caller cannot bypass the rule
// even if the handler forgets the check.
func (s *Service) ListDeleted(ctx context.Context, isAdmin bool) ([]*coredrive.Drive, error) {
	if !isAdmin {
		return nil, ErrPermission
	}
	return s.Drive.ListDeleted(ctx, time.Now(), 1000)
}
