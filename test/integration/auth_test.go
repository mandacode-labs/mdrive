package integration

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthMe_NoAuthConfigured(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	req := authReq("GET", srv.URL+"/auth/me", nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}
