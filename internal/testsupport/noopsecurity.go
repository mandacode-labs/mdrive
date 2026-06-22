// Package testsupport provides small helpers shared across the
// integration and e2e test suites. Keep this package free of domain
// dependencies so it can be imported from anywhere without cycles.
package testsupport

import (
	"context"

	"github.com/mandacode-labs/mdrive/pkg/api"
)

// NoopSecurity is an ogen SecurityHandler that accepts every bearer
// token without inspection. Use it in test servers that don't care
// about auth.
type NoopSecurity struct{}

// HandleBearerAuth returns the context unchanged.
func (NoopSecurity) HandleBearerAuth(ctx context.Context, _ api.OperationName, _ api.BearerAuth) (context.Context, error) {
	return ctx, nil
}
