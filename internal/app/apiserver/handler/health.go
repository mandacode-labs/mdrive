package handler

import (
	"context"
	"database/sql"

	"github.com/mandacode-labs/mdrive/internal/app/apputils"
	"github.com/mandacode-labs/mdrive/internal/auth/session"
	"github.com/mandacode-labs/mdrive/internal/permission"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

// HealthDeps captures the components the health check pings. nil values
// are skipped (useful in development where some backends are absent).
type HealthDeps struct {
	DB       *sql.DB
	ValKey   session.Scanner // presence indicates a Valkey-backed store
	Perm     permission.Checker
}

// Health returns a simple health check response. It returns 200 with
// status "ok" when all configured dependencies respond, or 503 with
// status "degraded" when any dependency is unreachable.
func (h *Handler) Health(ctx context.Context) (*api.HealthOK, error) {
	if h.healthDeps == nil {
		return &api.HealthOK{Status: apputils.OptString("ok")}, nil
	}
	if h.healthDeps.DB != nil {
		if err := h.healthDeps.DB.PingContext(ctx); err != nil {
			return &api.HealthOK{Status: apputils.OptString("degraded: database unreachable")}, nil
		}
	}
	if h.healthDeps.ValKey != nil {
		// Scan a single key to confirm connectivity; pass a no-op callback
		// so we don't actually iterate anything.
		if err := h.healthDeps.ValKey.Scan(ctx, func(_ string) error { return nil }); err != nil {
			return &api.HealthOK{Status: apputils.OptString("degraded: valkey unreachable")}, nil
		}
	}
	if h.healthDeps.Perm != nil {
		if _, err := h.healthDeps.Perm.Check(ctx, "healthcheck", "can_view", "drive", "_healthcheck"); err != nil {
			return &api.HealthOK{Status: apputils.OptString("degraded: openfga unreachable")}, nil
		}
	}
	return &api.HealthOK{Status: apputils.OptString("ok")}, nil
}
