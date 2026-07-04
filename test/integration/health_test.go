package integration

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealth(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/health")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "ok", body["status"])
}

// TestHealthAnonymousWithRealSecurity proves the production
// security handler does not gate /health. k8s liveness/readiness
// probes do not carry a session cookie, and the OpenAPI spec
// marks /health as `security: []`, but ogen 1.22 ignores the
// per-endpoint override when global security is set. The real
// SecurityHandler therefore short-circuits the health operation
// itself, and this test guards that rule. Without it, /health
// would return 401 in production and the k8s startup probe
// would fail (the /health 401 incident).
func TestHealthAnonymousWithRealSecurity(t *testing.T) {
	srv := newTestServerWith(t, realSecurityHandler{})
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/health")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"/health must be 200 even with the real (session-checking) security handler")
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "ok", body["status"])
}

// TestHealthAnonymousWithAuthBridge exercises the production
// middleware chain: OpenAPIPassthrough -> auth.Service.AuthBridge
// -> ogen. The /health 401 incident happened because AuthBridge
// rejected anonymous paths before the request reached the
// SecurityHandler, so testing the SecurityHandler alone (as
// TestHealthAnonymousWithRealSecurity does) is not enough -- this
// test guards the layer where the regression actually occurred.
func TestHealthAnonymousWithAuthBridge(t *testing.T) {
	srv := newTestServerWithAuthBridge(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/health")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"/health must be 200 through the full production middleware chain")
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "ok", body["status"])
}

// TestAuthenticatedRoute401WithoutCookie is the negative case for
// the cookie-auth flow: an authenticated route must 401 when the
// session cookie is missing, and the response must come from the
// real SecurityHandler (not a passthrough) so production behavior
// is exercised end-to-end.
func TestAuthenticatedRoute401WithoutCookie(t *testing.T) {
	srv := newTestServerWithAuthBridge(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/drives")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
		"/v1/drives must 401 without a session cookie")
}
