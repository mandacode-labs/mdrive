package handler

import (
	"context"

	apiv1 "github.com/mandacode-labs/mdrive/pkg/apiv1"
)

// Health returns a simple health check response.
func (h *Handler) Health(ctx context.Context) (*apiv1.HealthOK, error) {
	return &apiv1.HealthOK{Status: apistr("ok")}, nil
}
