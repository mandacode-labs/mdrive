package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/mdrive/api"
)

const testOpenAPISpec = `{
  "openapi": "3.1.0",
  "security": [{"bearerAuth": []}],
  "paths": {
    "/health": {
      "get": {
        "operationId": "health",
        "security": []
      }
    },
    "/auth/login": {
      "get": {
        "operationId": "authLogin",
        "security": []
      }
    },
    "/auth/callback": {
      "get": {
        "operationId": "authCallback",
        "security": []
      }
    },
    "/auth/logout": {
      "post": {
        "operationId": "authLogout",
        "security": []
      }
    },
    "/auth/me": {
      "get": {
        "operationId": "authMe"
      }
    },
    "/v1/drives": {
      "get": {
        "operationId": "listDrives"
      },
      "post": {
        "operationId": "createDrive",
        "security": [{"bearerAuth": []}]
      }
    }
  }
}`

func TestAnonymousPaths(t *testing.T) {
	got, err := anonymousPaths([]byte(testOpenAPISpec))
	require.NoError(t, err)

	for _, p := range []string{"/health", "/auth/login", "/auth/callback", "/auth/logout"} {
		assert.True(t, got[p], "%s must be in the anonymous set", p)
	}
	for _, p := range []string{"/auth/me", "/v1/drives"} {
		assert.False(t, got[p], "%s must NOT be in the anonymous set", p)
	}
}

// TestAnonymousPathsInvalidSpec makes sure a malformed spec
// surfaces as an error rather than silently producing an empty set,
// which would let AuthBridge lock down every public path.
func TestAnonymousPathsInvalidSpec(t *testing.T) {
	_, err := anonymousPaths([]byte("{not json"))
	require.Error(t, err)
}

// TestAnonymousPathsMatchesEmbeddedSpec guards against drift: if
// someone removes the `security: []` from /health in the bundled
// spec, this test fails. That would be the exact failure mode that
// caused the /health 401 incident.
func TestAnonymousPathsMatchesEmbeddedSpec(t *testing.T) {
	got, err := anonymousPaths(api.Spec)
	require.NoError(t, err)
	assert.True(t, got["/health"],
		"embedded api/openapi.bundled.json must declare /health as anonymous")
}
