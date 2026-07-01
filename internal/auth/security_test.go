package auth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/mdrive/internal/core/user"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

// stubUserSvc satisfies UserUpserter for the tests below.
type stubUserSvc struct {
	byProviderID func(ctx context.Context, provider, providerID string) (*user.User, error)
}

func (s *stubUserSvc) UpsertFromOIDC(ctx context.Context, cmd *user.CreateCommand) (*user.User, error) {
	return nil, errors.New("not used")
}

func (s *stubUserSvc) GetByProviderID(ctx context.Context, provider, providerID string) (*user.User, error) {
	return s.byProviderID(ctx, provider, providerID)
}

func newSecurityTestService(t *testing.T, users UserUpserter) *Service {
	t.Helper()
	return &Service{
		encKey:         newTestKey(t),
		cookieName:     "mdrive_session",
		providerName:   "keycloak",
		sessionTTL:     24 * time.Hour,
		users:          users,
	}
}

// TestHandleBearerAuthReturnsErrorx proves the regression fix:
// when the request has no session, HandleBearerAuth returns an
// errorx.KindUnauthenticated error, which ogen maps to 401 (not 500).
func TestHandleBearerAuthReturnsErrorx(t *testing.T) {
	svc := &Service{}
	sh := &SecurityHandler{auth: svc}

	ctx := context.Background()
	_, err := sh.HandleBearerAuth(ctx, "authMe", api.BearerAuth{Token: "ignored"})
	require.Error(t, err)

	var de errorx.Error
	require.True(t, errors.As(err, &de),
		"HandleBearerAuth must return an errorx.Error")
	assert.Equal(t, errorx.KindUnauthenticated, de.Kind())
	assert.Equal(t, http.StatusUnauthorized, de.Kind().Status())
}

// TestHandleBearerAuthPassesWithSession verifies the happy path is
// untouched by the errorx change.
func TestHandleBearerAuthPassesWithSession(t *testing.T) {
	svc := &Service{}
	sh := &SecurityHandler{auth: svc}

	sess := &Session{ID: "sub-123", UserID: "01HXYZ"}
	ctx := ContextWithSession(context.Background(), sess)

	out, err := sh.HandleBearerAuth(ctx, "authMe", api.BearerAuth{Token: "ignored"})
	require.NoError(t, err)
	assert.Equal(t, ctx, out, "ctx must be returned unchanged")
}

// TestHandleBearerAuthAllowsHealthAnonymous proves the regression
// fix for the /health 401 incident: k8s liveness/readiness
// probes do not carry a session, and the OpenAPI spec marks
// /health as `security: []`, but ogen 1.22 ignores that override
// when global security is set. The SecurityHandler therefore
// short-circuits the health operation so probes reach the
// handler that returns 200 (or 503 on degraded backends).
func TestHandleBearerAuthAllowsHealthAnonymous(t *testing.T) {
	svc := &Service{}
	sh := &SecurityHandler{auth: svc}

	ctx := context.Background() // no session
	out, err := sh.HandleBearerAuth(ctx, api.HealthOperation, api.BearerAuth{Token: "ignored"})
	require.NoError(t, err,
		"health operation must not require a session")
	assert.Equal(t, ctx, out, "ctx must be returned unchanged")
}

// TestAuthBridgeRejectsMissingCookie verifies AuthBridge now
// responds 401 directly when the session cookie is absent or
// invalid. Previously it passed the request through anonymously,
// which surfaced as 500 from ogen's NewError.
func TestAuthBridgeRejectsMissingCookie(t *testing.T) {
	svc := newSecurityTestService(t, nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/anything", nil)

	svc.AuthBridge(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next should not be called when session is invalid")
	})).ServeHTTP(w, r)

	assert.Equal(t, http.StatusUnauthorized, w.Code,
		"missing cookie must produce 401, not 500")
	body, _ := io.ReadAll(w.Body)
	assert.Contains(t, strings.TrimSpace(string(body)), "missing or invalid session cookie",
		"body must include the error reason")
}

// TestAuthBridgeRejectsUnknownUser verifies AuthBridge responds
// 401 when the session decrypts fine but the user is gone from
// the DB (cookie outlives the account, etc).
func TestAuthBridgeRejectsUnknownUser(t *testing.T) {
	users := &stubUserSvc{
		byProviderID: func(ctx context.Context, provider, providerID string) (*user.User, error) {
			return nil, nil // user not found
		},
	}
	svc := newSecurityTestService(t, users)

	// Plant a valid session cookie so the middleware gets past
	// the cookie-read step and into the user lookup.
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/anything", nil)
	sd := stateData{State: "matching", Verifier: "v"}
	raw, _ := json.Marshal(sd)
	enc, _ := encrypt(raw, svc.encKey)
	r.AddCookie(&http.Cookie{Name: svc.cookieName, Value: enc})

	svc.AuthBridge(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next should not be called when user is missing")
	})).ServeHTTP(w, r)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	body, _ := io.ReadAll(w.Body)
	assert.Contains(t, strings.TrimSpace(string(body)), "user not found")
}

// TestAuthBridgeAttachesSessionOnSuccess verifies the happy path
// still sets Authorization and attaches the session to ctx.
func TestAuthBridgeAttachesSessionOnSuccess(t *testing.T) {
	users := &stubUserSvc{
		byProviderID: func(ctx context.Context, provider, providerID string) (*user.User, error) {
			return user.NewUser("user-1", "pub-1", "Alice", nil, provider, providerID,
				timeFromUnix(0), timeFromUnix(0)), nil
		},
	}
	svc := newSecurityTestService(t, users)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/anything", nil)
	sd := stateData{State: "matching", Verifier: "v"}
	raw, _ := json.Marshal(sd)
	enc, _ := encrypt(raw, svc.encKey)
	r.AddCookie(&http.Cookie{Name: svc.cookieName, Value: enc})

	var seenSess *Session
	var seenAuth string
	svc.AuthBridge(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenSess = SessionFromContext(r.Context())
		seenAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, seenSess, "SessionFromContext must return non-nil")
	assert.Equal(t, "user-1", seenSess.UserID)
	assert.Equal(t, "Bearer user-1", seenAuth)
}

// timeFromUnix is a tiny helper so the test does not import time
// just for one value.
func timeFromUnix(sec int64) time.Time { return time.Unix(sec, 0) }

// TestAuthBridgeAllowsAnonymousPath proves the regression fix for
// the /health 401 incident. AuthBridge must consult the spec-derived
// anonymous set (s.noAuthPaths) before the cookie check, so paths
// declared `security: []` in the OpenAPI spec are not gated by the
// session cookie. The set is populated by New() from the embedded
// api.Spec; here we set it directly to keep the test independent
// of the bundled spec contents.
func TestAuthBridgeAllowsAnonymousPath(t *testing.T) {
	svc := newSecurityTestService(t, nil)
	svc.noAuthPaths = map[string]bool{"/health": true}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/health", nil)

	called := false
	svc.AuthBridge(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	})).ServeHTTP(w, r)

	assert.True(t, called,
		"next must be called for an anonymous path with no cookie")
	assert.NotEqual(t, http.StatusUnauthorized, w.Code,
		"anonymous paths must not produce 401")
}

// TestAuthBridgeRejectsAuthenticatedPath makes sure adding a path
// to the anonymous set is the ONLY way to make AuthBridge skip it.
// /v1/drives is authenticated; even with noAuthPaths empty, a
// missing cookie must produce 401, not pass through.
func TestAuthBridgeRejectsAuthenticatedPath(t *testing.T) {
	svc := newSecurityTestService(t, nil)
	svc.noAuthPaths = map[string]bool{"/health": true}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/v1/drives", nil)

	svc.AuthBridge(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next should not be called for authenticated path without cookie")
	})).ServeHTTP(w, r)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}