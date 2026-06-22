package apiserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

type ctxKey int

const (
	ctxKeyRequestID ctxKey = iota
)

const requestIDHeader = "X-Request-Id"

// RequestIDMiddleware ensures every request has a request ID in its
// context. Inbound X-Request-Id is reused when present, otherwise a new
// 16-byte hex ID is generated. The ID is echoed back in the response
// header and stored in the context for downstream handlers and logs.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(requestIDHeader)
		if id == "" {
			generated, err := randomHex(8)
			if err != nil {
				// crypto/rand should not fail on a healthy system; if
				// it does, the request is broken either way. Surface a
				// 500 rather than smuggling a guessable ID into logs.
				http.Error(w, "request id unavailable", http.StatusInternalServerError)
				return
			}
			id = generated
		}
		w.Header().Set(requestIDHeader, id)
		ctx := context.WithValue(r.Context(), ctxKeyRequestID, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFromContext returns the request ID stored in ctx, or "" if
// none is present.
func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyRequestID).(string); ok {
		return v
	}
	return ""
}

// randomHex returns a hex-encoded string of n random bytes.
func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
