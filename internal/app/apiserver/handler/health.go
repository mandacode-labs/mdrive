package handler

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/mandacode-labs/mdrive/internal/app/apiopts"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/permission"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

type ValkeyScanner interface {
	Scan(ctx context.Context, fn func(string) error) error
}

type HealthDeps struct {
	DB         *sql.DB
	Valkey     ValkeyScanner
	Authorizer permission.Authorizer
}

func (h *Handler) Health(ctx context.Context) (api.HealthRes, error) {
	if h.healthDeps.DB != nil {
		if err := h.healthDeps.DB.PingContext(ctx); err != nil {
			return nil, errorx.New(errorx.KindServiceDegraded, fmt.Sprintf("health: database ping failed (err=%v)", err))
		}
	}
	if h.healthDeps.Valkey != nil {
		if err := h.healthDeps.Valkey.Scan(ctx, func(_ string) error { return nil }); err != nil {
			return nil, errorx.New(errorx.KindServiceDegraded, fmt.Sprintf("health: valkey scan failed (err=%v)", err))
		}
	}
	if h.healthDeps.Authorizer != nil {
		if _, err := h.healthDeps.Authorizer.Check(ctx, "healthcheck", permission.ActionView, "drive", "_healthcheck"); err != nil {
			return nil, errorx.New(errorx.KindServiceDegraded, fmt.Sprintf("health: openfga check failed (err=%v)", err))
		}
	}
	return &api.HealthOK{Status: apiopts.OptString("ok")}, nil
}
