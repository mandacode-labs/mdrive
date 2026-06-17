package handler

import (
	"context"

	"github.com/mandacode-labs/mdrive/pkg/api"
)

// Health returns a simple health check response.
func (h *Handler) Health(ctx context.Context) (*api.HealthOK, error) {
	return &api.HealthOK{Status: apistr("ok")}, nil
}
