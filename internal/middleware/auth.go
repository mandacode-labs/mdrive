package middleware

import (
	"context"
	"net/http"

	"github.com/mandacode-labs/retrowin-go/internal/errors"
	"github.com/mandacode-labs/retrowin-go/internal/utils"
)

// GetUserID extracts user ID from context.
// Deprecated: Use utils.GetUserID instead.
func GetUserID(ctx context.Context) string {
	id, _ := utils.GetUserID(ctx)
	return id
}

// GetSessionID extracts session ID from context.
// Deprecated: Use utils.GetSessionID instead.
func GetSessionID(ctx context.Context) string {
	id, _ := utils.GetSessionID(ctx)
	return id
}

// SetUserID adds user ID to context.
// Deprecated: Use utils.ContextWithUserID instead.
func SetUserID(ctx context.Context, id string) context.Context {
	return utils.ContextWithUserID(ctx, id)
}

// SetSessionID adds session ID to context.
// Deprecated: Use utils.ContextWithSession instead.
func SetSessionID(ctx context.Context, id string) context.Context {
	return utils.ContextWithSession(ctx, id)
}

// WriteError writes an error response.
func WriteError(w http.ResponseWriter, err *errors.Error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.StatusCode)
	_, _ = w.Write([]byte(`{"code":"` + err.Code + `","message":"` + err.Message + `"}`))
}
