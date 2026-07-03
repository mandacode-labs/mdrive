package handler

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	handlerMocks "github.com/mandacode-labs/mdrive/internal/app/apiserver/handler/mocks"
	"github.com/mandacode-labs/mdrive/internal/auth"
	"github.com/mandacode-labs/mdrive/internal/core/user"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

// authUserIDContext attaches a real auth.Session with just a
// userID populated. The session has no Subject/Provider — that's
// fine because h.userID only reads sess.UserID.
func authUserIDContext(userID string) context.Context {
	return auth.ContextWithSession(context.Background(), &auth.Session{
		ID:        "sub-test",
		UserID:    userID,
		Provider:  "keycloak",
		ExpiresAt: time.Now().Add(time.Hour),
	})
}

// requireAuthMeErr asserts that res is an *api.Error carrying
// the given kind.
func requireAuthMeErr(t *testing.T, res api.AuthMeRes, want errorx.Kind) {
	t.Helper()
	require.NotNil(t, res)
	apiErr, ok := res.(*api.Error)
	require.True(t, ok, "expected *api.Error, got %T", res)
	assert.Equal(t, api.ErrorCode(want.String()), apiErr.Code,
		"unexpected error code in response")
}

// newUserMockThatReturns wires GetByID to return the given
// (user, err). Other methods are not configured; mockery's
// Cleanup-AssertExpectations rejects unexpected calls only for
// methods that had On/Return registered, so callers that don't
// invoke UpsertFromOIDC see no failure.
func newUserMockThatReturns(t *testing.T, u *user.User, err error) *handlerMocks.UserClientMock {
	t.Helper()
	m := handlerMocks.NewUserClientMock(t)
	m.EXPECT().GetByID(mock.Anything, mock.Anything).Return(u, err).Maybe()
	return m
}

// TestAuthMeReturns401WhenNoUser proves the regression: when no
// session is attached, AuthMe returns *api.Error with
// KindUnauthenticated rather than passing silently through to
// ogen and surfacing as a 500.
func TestAuthMeReturns401WhenNoUser(t *testing.T) {
	users := newUserMockThatReturns(t, nil, errorx.New(errorx.KindUnauthenticated, "no session"))
	h := &Handler{
		users:       users,
		redirectURI: "https://api.example.com/auth/callback",
	}

	res, err := h.AuthMe(context.Background())
	require.NoError(t, err)
	requireAuthMeErr(t, res, errorx.KindUnauthenticated)
}

func TestAuthMeReturns503OnDBError(t *testing.T) {
	users := newUserMockThatReturns(t, nil, errorx.New(errorx.KindServiceDegraded, "connection refused"))
	h := &Handler{
		users:       users,
		redirectURI: "https://api.example.com/auth/callback",
	}

	res, err := h.AuthMe(authUserIDContext("user-1"))
	require.NoError(t, err)
	requireAuthMeErr(t, res, errorx.KindServiceDegraded)
}

func TestAuthMeReturns404OnMissingUser(t *testing.T) {
	users := newUserMockThatReturns(t, nil, nil)
	h := &Handler{
		users:       users,
		redirectURI: "https://api.example.com/auth/callback",
	}

	res, err := h.AuthMe(authUserIDContext("user-1"))
	require.NoError(t, err)
	requireAuthMeErr(t, res, errorx.KindNotFound)
}

func TestAuthMeReturns200WithUser(t *testing.T) {
	now := time.Unix(1700000000, 0)
	u := user.NewUser("user-1", "pub-1", "Alice", nil, "keycloak", "sub-1", now, now)
	users := newUserMockThatReturns(t, u, nil)
	h := &Handler{
		users:       users,
		redirectURI: "https://api.example.com/auth/callback",
	}

	res, err := h.AuthMe(authUserIDContext("user-1"))
	require.NoError(t, err)
	require.NotNil(t, res)
	got, ok := res.(*api.User)
	require.True(t, ok, "expected *api.User, got %T", res)
	assert.True(t, got.ID.Set, "id must be set on success")
	assert.Equal(t, "user-1", got.ID.Value)
}
