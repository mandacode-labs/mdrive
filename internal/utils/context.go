// Package utils provides shared context helpers.
package utils

import "context"

// Private context-key type to avoid collisions.
type contextKey int

const (
	userIDKey contextKey = iota
	sessionIDKey
)

// WithUserID returns a copy of ctx with the user ID stored in it.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// UserID returns the user ID from the context, if any.
func UserID(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(userIDKey).(string)
	return v, ok
}

// WithSessionID returns a copy of ctx with the session ID stored in it.
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, sessionIDKey, sessionID)
}

// SessionID returns the session ID from the context, if any.
func SessionID(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(sessionIDKey).(string)
	return v, ok
}
