package integration

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAuthMe_NoAuthConfigured(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	req := authReq("GET", srv.URL+"/auth/me", nil)
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}
