package apiserver

import (
	"context"

	"github.com/mandacode-labs/mdrive/pkg/api"
)

// AnonSecurity is the development / no-auth default SecurityHandler.
// It accepts every request without inspection and emits a
// startup-time warning so operators do not accidentally deploy it to
// production. Use the real *auth.Service (wired by NewServer when
// cfg.Auth is configured) in any environment that handles real data.
type AnonSecurity struct{}

// HandleCookieAuth returns the context unchanged. No session lookup,
// no expiration check, no audit trail.
func (AnonSecurity) HandleCookieAuth(ctx context.Context, _ api.OperationName, _ api.CookieAuth) (context.Context, error) {
	return ctx, nil
}
