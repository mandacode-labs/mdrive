// Package apiserver builds the HTTP server that exposes the
// generated ogen handler, plus the OpenAPI spec and OIDC flow.
//
// Cross-cutting middlewares live in ./middleware; path-specific
// routes (OIDC flow, /openapi.json) are mounted on the static
// router inside buildChain in server.go. This file is intentionally
// kept minimal so the composition shape (what wraps what, and why)
// is readable from server.go alone.
package apiserver
