package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/mdrive/internal/core/user"
	userMocks "github.com/mandacode-labs/mdrive/internal/core/user/mocks"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

func newSecurityTestService(t *testing.T, users user.Upserter) *Service {
	t.Helper()
	return &Service{
		encKey:       newTestKey(t),
		cookieName:   "mdrive_session",
		providerName: "keycloak",
		sessionTTL:   24 * time.Hour,
		users:        users,
	}
}

// TestHandleCookieAuthRejectsEmptyAPIKey proves the regression fix:
// when the cookie value is empty, HandleCookieAuth returns an
// errorx.KindUnauthenticated error, which ogen maps to 401 (not 500).
func TestHandleCookieAuthRejectsEmptyAPIKey(t *testing.T) {
	svc := &Service{}

	ctx := context.Background()
	_, err := svc.HandleCookieAuth(ctx, api.AuthMeOperation, api.CookieAuth{APIKey: ""})
	require.Error(t, err)

	var de errorx.Error
	require.True(t, errors.As(err, &de),
		"HandleCookieAuth must return an errorx.Error")
	assert.Equal(t, errorx.KindUnauthenticated, de.Kind())
	assert.Equal(t, http.StatusUnauthorized, de.Kind().Status())
}

// TestHandleCookieAuthRejectsInvalidCookie verifies that a malformed
// cookie payload (decryption fails) returns 401, not 500.
func TestHandleCookieAuthRejectsInvalidCookie(t *testing.T) {
	svc := newSecurityTestService(t, newUserUpserterMock(t, nil, errorx.New(errorx.KindNotFound, "")))

	ctx := context.Background()
	_, err := svc.HandleCookieAuth(ctx, api.AuthMeOperation, api.CookieAuth{APIKey: "not-a-real-encrypted-cookie"})
	require.Error(t, err)

	var de errorx.Error
	require.True(t, errors.As(err, &de),
		"HandleCookieAuth must return an errorx.Error")
	assert.Equal(t, errorx.KindUnauthenticated, de.Kind())
}

// TestHandleCookieAuthAttachesSessionOnSuccess verifies the happy path
// resolves the user from the cookie's subject claim and attaches a
// Session to ctx.
func TestHandleCookieAuthAttachesSessionOnSuccess(t *testing.T) {
	u := user.NewUser("user-1", "pub-1", "Alice", nil, "keycloak", "sub-1",
		timeFromUnix(0), timeFromUnix(0))
	svc := newSecurityTestService(t, newUserUpserterMock(t, u, nil))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/anything", nil)
	sd := sessionData{Subject: "sub-1", UserID: "user-1", Provider: "keycloak", ExpiresAt: time.Now().Add(time.Hour)}
	raw, _ := json.Marshal(sd)
	enc, _ := encrypt(raw, svc.encKey)
	r.AddCookie(&http.Cookie{Name: svc.cookieName, Value: enc})

	ctx := context.Background()
	out, err := svc.HandleCookieAuth(ctx, api.AuthMeOperation, api.CookieAuth{APIKey: enc})
	require.NoError(t, err)
	sess := SessionFromContext(out)
	require.NotNil(t, sess, "Session must be attached to ctx")
	assert.Equal(t, "user-1", sess.UserID)
	assert.Equal(t, "keycloak", sess.Provider)
	_ = w
	_ = r
}

// TestHandleCookieAuthRejectsExpiredCookie verifies that an expired
// session cookie returns 401, not 500. The expired-cookie path is
// distinct from a malformed-cookie path because both produce
// different errorx frames; the middleware should treat them
// uniformly.
func TestHandleCookieAuthRejectsExpiredCookie(t *testing.T) {
	svc := newSecurityTestService(t, newUserUpserterMock(t, nil, errorx.New(errorx.KindNotFound, "")))

	sd := sessionData{Subject: "sub-1", Provider: "keycloak", ExpiresAt: time.Now().Add(-time.Hour)}
	raw, _ := json.Marshal(sd)
	enc, _ := encrypt(raw, svc.encKey)

	_, err := svc.HandleCookieAuth(context.Background(), api.AuthMeOperation, api.CookieAuth{APIKey: enc})
	require.Error(t, err)

	var de errorx.Error
	require.True(t, errors.As(err, &de))
	assert.Equal(t, errorx.KindUnauthenticated, de.Kind(),
		"expired cookie must surface as unauthenticated")
}

// TestHandleCookieAuthRejectsUnknownUser verifies that a valid cookie
// for a user that no longer exists in the DB returns 401.
func TestHandleCookieAuthRejectsUnknownUser(t *testing.T) {
	users := newUserUpserterMock(t, nil, errorx.New(errorx.KindNotFound, "user not found"))
	svc := newSecurityTestService(t, users)

	sd := sessionData{Subject: "sub-1", Provider: "keycloak", ExpiresAt: time.Now().Add(time.Hour)}
	raw, _ := json.Marshal(sd)
	enc, _ := encrypt(raw, svc.encKey)

	_, err := svc.HandleCookieAuth(context.Background(), api.AuthMeOperation, api.CookieAuth{APIKey: enc})
	require.Error(t, err)

	var de errorx.Error
	require.True(t, errors.As(err, &de))
	assert.Equal(t, errorx.KindUnauthenticated, de.Kind())
}

// newUserUpserterMock returns an UpserterMock whose GetByProviderID
// returns (user, nil) and whose UpsertFromOIDC is expected but unused.
// mockery's per-method Maybe() ensures a test that never calls
// UpsertFromOIDC still passes cleanup.
func newUserUpserterMock(t *testing.T, u *user.User, getErr error) *userMocks.UpserterMock {
	t.Helper()
	m := userMocks.NewUpserterMock(t)
	m.EXPECT().GetByProviderID(mock.Anything, mock.Anything, mock.Anything).
		Return(u, getErr).Maybe()
	m.EXPECT().UpsertFromOIDC(mock.Anything, mock.Anything).
		Return(nil, errors.New("not used")).Maybe()
	return m
}

// timeFromUnix is a tiny helper so the test does not import time
// just for one value.
func timeFromUnix(sec int64) time.Time { return time.Unix(sec, 0) }
