package handler

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/logx"
	"github.com/mandacode-labs/mdrive/internal/permission"
	"github.com/mandacode-labs/mdrive/internal/upload"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

type HealthDeps struct {
	DB         *sql.DB
	Valkey     upload.TokenScanner
	Authorizer permission.Authorizer
}

func (h *Handler) Health(ctx context.Context) (api.HealthRes, error) {
	logx.Debug(ctx, "handler.health.enter")
	if h.healthDeps.DB != nil {
		if err := h.healthDeps.DB.PingContext(ctx); err != nil {
			logx.Debug(ctx, "handler.health.db_ping_err", slog.String("err", err.Error()))
			return nil, errorx.New(errorx.KindUnavailable, fmt.Sprintf("health: database ping failed (err=%v)", err))
		}
	}
	if h.healthDeps.Valkey != nil {
		if err := h.healthDeps.Valkey.Scan(ctx, func(_ string) error { return nil }); err != nil {
			logx.Debug(ctx, "handler.health.valkey_scan_err", slog.String("err", err.Error()))
			return nil, errorx.New(errorx.KindUnavailable, fmt.Sprintf("health: valkey scan failed (err=%v)", err))
		}
	}
	if h.healthDeps.Authorizer != nil {
		if _, err := h.healthDeps.Authorizer.Check(ctx, "healthcheck", permission.ActionView, permission.ObjectTypeDrive, "_healthcheck"); err != nil {
			logx.Debug(ctx, "handler.health.fga_err", slog.String("err", err.Error()))
			return nil, errorx.New(errorx.KindUnavailable, fmt.Sprintf("health: openfga check failed (err=%v)", err))
		}
	}
	logx.Debug(ctx, "handler.health.ok")
	return &api.HealthOK{Status: optString("ok")}, nil
}
