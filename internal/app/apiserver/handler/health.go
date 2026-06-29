package handler

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/mandacode-labs/mdrive/internal/app/apiopts"
	"github.com/mandacode-labs/mdrive/internal/auth/session"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/permission"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

var ErrServiceDegraded = &errorx.Error{Kind: errorx.KindServiceDegraded, Msg: "service degraded"}

// HealthDeps captures the components the health check pings. nil values
// are skipped (useful in development where some backends are absent).
type HealthDeps struct {
	DB         *sql.DB
	Valkey     session.Scanner
	Authorizer permission.Authorizer
}

// Health returns 200 with status "ok" when all configured
// dependencies respond, or an error wrapping ErrServiceDegraded
// (mapped to 503 by FromError) when any dependency is unreachable.
// A zero-value HealthDeps is treated as 'no dependencies
// configured' and always returns ok.
func (h *Handler) Health(ctx context.Context) (*api.HealthOK, error) {
	if h.healthDeps.DB != nil {
		if err := h.healthDeps.DB.PingContext(ctx); err != nil {
			return nil, fmt.Errorf("%w: database: %s", ErrServiceDegraded, err.Error())
		}
	}
	if h.healthDeps.Valkey != nil {
		// Scan with a no-op callback to confirm connectivity without
		// actually iterating anything.
		if err := h.healthDeps.Valkey.Scan(ctx, func(_ string) error { return nil }); err != nil {
			return nil, fmt.Errorf("%w: valkey: %s", ErrServiceDegraded, err.Error())
		}
	}
	if h.healthDeps.Authorizer != nil {
		if _, err := h.healthDeps.Authorizer.Check(ctx, "healthcheck", permission.ActionView, "drive", "_healthcheck"); err != nil {
			return nil, fmt.Errorf("%w: openfga: %s", ErrServiceDegraded, err.Error())
		}
	}
	return &api.HealthOK{Status: apiopts.OptString("ok")}, nil
}
