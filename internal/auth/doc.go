// Package auth provides OIDC authentication middleware and
// encrypted cookie-based session management backed by Keycloak.
//
// The flow is OpenID Connect Authorization Code + PKCE. Sessions
// are encrypted with AES-GCM and stored in a single cookie. There
// is no server-side session store.
package auth
