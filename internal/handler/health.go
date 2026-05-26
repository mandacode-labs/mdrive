package handler

import (
	"context"

	api "github.com/mandacode-labs/retrowin-go/pkg/api"
)

// GetHealth implements GET /health.
func (h *Handler) GetHealth(ctx context.Context) (api.GetHealthRes, error) {
	return &api.HealthStatus{
		Status: api.HealthStatusStatusHealthy,
	}, nil
}
