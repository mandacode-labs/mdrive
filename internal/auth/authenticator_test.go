package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/mdrive/internal/auth/session"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, ".well-known") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"issuer":"http://` + r.Host + `",
				"authorization_endpoint":"http://` + r.Host + `/authorize",
				"token_endpoint":"http://` + r.Host + `/token",
				"userinfo_endpoint":"http://` + r.Host + `/userinfo",
				"end_session_endpoint":"http://` + r.Host + `/end_session",
				"jwks_uri":"http://` + r.Host + `/keys"
			}`))
		}
	}))
}

func newTestService(t *testing.T, srv *httptest.Server) *Service {
	t.Helper()
	a, err := NewService(context.Background(), Config{
		Issuer:       "http://" + srv.Listener.Addr().String(),
		ClientID:     "test-client",
		SessionStore: session.NewMemoryStore(),
		SessionTTL:   1 * time.Hour,
	})
	require.NoError(t, err)
	return a
}

func TestAuthorizeURL(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	a := newTestService(t, srv)

	url, err := a.AuthorizeURL(context.Background(), "google", "http://app.local/callback", "state123", "challenge456")
	require.NoError(t, err)

	assert.Contains(t, url, "client_id=test-client")
	assert.Contains(t, url, "response_type=code")
	assert.Contains(t, url, "scope=openid+profile+email")
	assert.Contains(t, url, "redirect_uri=http%3A%2F%2Fapp.local%2Fcallback")
	assert.Contains(t, url, "state=state123")
	assert.Contains(t, url, "code_challenge_method=S256")
	assert.Contains(t, url, "code_challenge=challenge456")
	assert.Contains(t, url, "idp_id=google")
}

func TestStoreAndGetPKCE(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	a := newTestService(t, srv)

	err := a.StorePKCE(context.Background(), "state123", "verifier456")
	require.NoError(t, err)

	got, err := a.GetPKCE(context.Background(), "state123")
	require.NoError(t, err)
	assert.Equal(t, "verifier456", got)

	_, err = a.GetPKCE(context.Background(), "state123")
	assert.Error(t, err)
}

func TestCreateAndDeleteSession(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	a := newTestService(t, srv)

	sess, err := a.CreateSession(context.Background(), "user123", "google", false)
	require.NoError(t, err)
	assert.Equal(t, "user123", sess.UserID)
	assert.Equal(t, "google", sess.Provider)
	assert.False(t, sess.IsAdmin)
	assert.WithinDuration(t, time.Now().Add(1*time.Hour), sess.ExpiresAt, 5*time.Second)

	err = a.DeleteSession(context.Background(), sess.ID)
	require.NoError(t, err)
}
