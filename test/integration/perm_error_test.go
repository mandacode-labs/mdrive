package integration

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/mdrive/internal/app/apiserver/handler"
	"github.com/mandacode-labs/mdrive/internal/app/apiserver/middleware"
	permissionMocks "github.com/mandacode-labs/mdrive/internal/permission/mocks"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

// TestPermissionCheckTransportErrorReturns5xx is the regression
// test for the original mkdir 200 incident: a transport-level
// failure inside the OpenFGA Check must surface as 5xx, never 200.
//
// We exercise the production middleware chain (ErrorMiddleware
// + NewError) against an in-memory mock authorizer that always
// returns a non-errorx error. requirePerm must wrap it as
// KindServiceDegraded, and the wire response must be 503.
func TestPermissionCheckTransportErrorReturns5xx(t *testing.T) {
	authz := permissionMocks.NewAuthorizerMock(t)
	authz.On("Check", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(false, errors.New("openfga: POST validation error: invalid user")).
		Maybe() // allow zero calls
	authz.On("Grant", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Maybe()
	authz.On("Revoke", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Maybe()

	h := handler.New(
		zeroFS{},
		zeroDrive{owner: testUserID},
		newFakeUserSvc(),
		zeroUpload{},
		authz,
		"",
	)
	sec := testSecurity{}
	ogenServer, err := api.NewServer(h, sec,
		api.WithMiddleware(middleware.ErrorMiddleware(), middleware.PanicMiddleware()),
	)
	require.NoError(t, err)
	mux := http.NewServeMux()
	mux.Handle("/", ogenServer)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	req, _ := http.NewRequest("POST", srv.URL+"/v1/drives/01HXX/fs/mkdir",
		bytes.NewBufferString(`{"path":"/x"}`))
	req.AddCookie(&http.Cookie{Name: "mdrive_session", Value: "bypass"})
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode,
		"transport-level permission check failure must surface as 503 (got %d)", resp.StatusCode)
}

// TestPermissionDeniedReturns403 covers the negative path: an
// authorized request that the authorizer rejects must surface
// as 403, not 200. This is the second branch of requirePerm.
func TestPermissionDeniedReturns403(t *testing.T) {
	authz := permissionMocks.NewAuthorizerMock(t)
	authz.On("Check", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(false, nil).Maybe()
	authz.On("Grant", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Maybe()
	authz.On("Revoke", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Maybe()

	h := handler.New(
		zeroFS{},
		zeroDrive{owner: testUserID},
		newFakeUserSvc(),
		zeroUpload{},
		authz,
		"",
	)
	sec := testSecurity{}
	ogenServer, err := api.NewServer(h, sec,
		api.WithMiddleware(middleware.ErrorMiddleware(), middleware.PanicMiddleware()),
	)
	require.NoError(t, err)
	mux := http.NewServeMux()
	mux.Handle("/", ogenServer)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	req, _ := http.NewRequest("POST", srv.URL+"/v1/drives/01HXX/fs/mkdir",
		bytes.NewBufferString(`{"path":"/x"}`))
	req.AddCookie(&http.Cookie{Name: "mdrive_session", Value: "bypass"})
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusForbidden, resp.StatusCode,
		"permission denial must surface as 403 (got %d)", resp.StatusCode)
}
