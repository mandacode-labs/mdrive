package handler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/mdrive/internal/auth"
	"github.com/mandacode-labs/mdrive/internal/core/user"
	userMocks "github.com/mandacode-labs/mdrive/internal/core/user/mocks"
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

// requireAuthMeErrKind asserts that err's chain carries an
// errorx.Error with the given kind. The middleware path converts
// this to the right status.
func requireAuthMeErrKind(t *testing.T, err error, want errorx.Kind) {
	t.Helper()
	require.Error(t, err)
	assert.True(t, errorx.IsKind(err, want), "expected kind=%s, got %s", want, errorx.KindOf(err))
}

// newUserMockThatReturns wires GetByID to return the given
// (user, err). Other methods are not configured; mockery's
// Cleanup-AssertExpectations rejects unexpected calls only for
// methods that had On/Return registered, so callers that don't
// invoke UpsertFromOIDC see no failure.
func newUserMockThatReturns(t *testing.T, u *user.User, err error) *userMocks.ServiceMock {
	t.Helper()
	m := userMocks.NewServiceMock(t)
	m.EXPECT().GetByID(mock.Anything, mock.Anything).Return(u, err).Maybe()
	return m
}

// TestAuthMeReturnsErrWhenNoSession covers the unauthenticated
// branch. The error path returns the errorx so the middleware
// (kindToCode) writes 401.
func TestAuthMeReturnsErrWhenNoSession(t *testing.T) {
	users := newUserMockThatReturns(t, nil, nil)
	h := &Handler{
		users:       users,
		redirectURI: "https://api.example.com/auth/callback",
	}

	res, err := h.AuthMe(context.Background())
	require.Nil(t, res)
	requireAuthMeErrKind(t, err, errorx.KindUnauthenticated)
}

func TestAuthMeReturnsErrOnDBError(t *testing.T) {
	users := newUserMockThatReturns(t, nil, errors.New("connection refused"))
	h := &Handler{
		users:       users,
		redirectURI: "https://api.example.com/auth/callback",
	}

	res, err := h.AuthMe(authUserIDContext("user-1"))
	require.Nil(t, res)
	requireAuthMeErrKind(t, err, errorx.KindUnavailable)
}

func TestAuthMeReturnsErrOnMissingUser(t *testing.T) {
	users := newUserMockThatReturns(t, nil, nil)
	h := &Handler{
		users:       users,
		redirectURI: "https://api.example.com/auth/callback",
	}

	res, err := h.AuthMe(authUserIDContext("user-1"))
	require.Nil(t, res)
	requireAuthMeErrKind(t, err, errorx.KindNotFound)
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
