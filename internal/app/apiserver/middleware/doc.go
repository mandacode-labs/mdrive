// Package middleware holds the cross-cutting middlewares. Path-
// specific routes (OIDC flow, /openapi.json) live on the mux in
// server.go — they are not middlewares.
package middleware
