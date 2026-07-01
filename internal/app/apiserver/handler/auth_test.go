package handler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/mdrive/internal/auth"
	"github.com/mandacode-labs/mdrive/internal/core/user"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

// stubUsersForAuth satisfies UserClient for the AuthMe tests.
type stubUsersForAuth struct {
	getByID func(ctx context.Context, id string) (*user.User, error)
}

func (s *stubUsersForAuth) UpsertFromOIDC(ctx context.Context, cmd *user.CreateCommand) (*user.User, error) {
	return nil, errors.New("unused")
}

func (s *stubUsersForAuth) GetByID(ctx context.Context, id string) (*user.User, error) {
	return s.getByID(ctx, id)
}

// authUserIDContext attaches a real auth.Session with just a userID
// populated. The session has no Subject/Provider -- that's fine
// because h.userID only reads sess.UserID.
func authUserIDContext(userID string) context.Context {
	return auth.ContextWithSession(context.Background(), &auth.Session{
		ID:        "sub-test",
		UserID:    userID,
		Provider:  "keycloak",
		ExpiresAt: time.Now().Add(time.Hour),
	})
}

// requireAuthMeErr asserts that res is an *api.Error carrying
// the given kind. AuthMe dispatches errors via the AuthMeRes
// interface (the second return is always nil), so error-kind
// checks live in the response type.
func requireAuthMeErr(t *testing.T, res api.AuthMeRes, want errorx.Kind) {
	t.Helper()
	require.NotNil(t, res)
	apiErr, ok := res.(*api.Error)
	require.True(t, ok, "expected *api.Error, got %T", res)
	assert.Equal(t, api.ErrorCode(want.String()), apiErr.Code,
		"unexpected error code in response")
}

// TestAuthMeReturns401WhenNoUser proves the regression: when no
// session is attached, AuthMe returns *api.Error with
// KindUnauthenticated rather than passing silently through to
// ogen and surfacing as a 500.
func TestAuthMeReturns401WhenNoUser(t *testing.T) {
	h := &Handler{
		users: &stubUsersForAuth{getByID: func(ctx context.Context, id string) (*user.User, error) {
			t.Fatal("GetByID must not be called when there is no user in context")
			return nil, nil
		}},
		redirectURI: "https://api.example.com/auth/callback",
	}

	res, err := h.AuthMe(context.Background())
	require.NoError(t, err)
	requireAuthMeErr(t, res, errorx.KindUnauthenticated)
}

func TestAuthMeReturns503OnDBError(t *testing.T) {
	h := &Handler{
		users: &stubUsersForAuth{getByID: func(ctx context.Context, id string) (*user.User, error) {
			return nil, errors.New("connection refused")
		}},
		redirectURI: "https://api.example.com/auth/callback",
	}

	res, err := h.AuthMe(authUserIDContext("user-1"))
	require.NoError(t, err)
	requireAuthMeErr(t, res, errorx.KindServiceDegraded)
}

func TestAuthMeReturns404OnMissingUser(t *testing.T) {
	h := &Handler{
		users: &stubUsersForAuth{getByID: func(ctx context.Context, id string) (*user.User, error) {
			return nil, nil
		}},
		redirectURI: "https://api.example.com/auth/callback",
	}

	res, err := h.AuthMe(authUserIDContext("user-1"))
	require.NoError(t, err)
	requireAuthMeErr(t, res, errorx.KindNotFound)
}

func TestAuthMeReturns200WithUser(t *testing.T) {
	now := time.Unix(1700000000, 0)
	h := &Handler{
		users: &stubUsersForAuth{getByID: func(ctx context.Context, id string) (*user.User, error) {
			return user.NewUser("user-1", "pub-1", "Alice", nil, "keycloak", "sub-1", now, now), nil
		}},
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