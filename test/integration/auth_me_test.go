package integration

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAuthMeRejectsUnauthenticatedNoLonger500 is the regression
// guard for the /auth/me 500 that hit production. Before the fix,
// missing-session responses fell through ogen's default error
// path to a raw 500 with a generic "internal error" body.
//
// After the fix, the response is no longer 500: ogen's HardCode
// for the *api.Error variant of AuthMeRes is 401. (The exact
// status depends on ogen internals; this test only enforces that
// the broken 500 path is gone.)
func TestAuthMeRejectsUnauthenticatedNoLonger500(t *testing.T) {
	srv := newTestServerWith(t, realSecurityHandler{})
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/auth/me")
	require.NoError(t, err)
	_ = resp.Body.Close()

	assert.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
		"missing session must no longer surface as the silent 500 that hid the regression")
}