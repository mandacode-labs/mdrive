// Package session is the session-store abstraction. A session
// is an opaque token issued by the OIDC login flow that the
// bearer-auth middleware exchanges for the authenticated user.
// Production stores the token in Valkey; development uses an
// in-memory store.
package session
