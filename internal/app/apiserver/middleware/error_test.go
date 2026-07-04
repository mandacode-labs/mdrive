package middleware

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/ogen-go/ogen/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

func TestKindToCode_Nil(t *testing.T) {
	assert.Nil(t, KindToCode(nil))
}

func TestKindToCode_KindFromChain(t *testing.T) {
	wrapped := errorx.Wrap(errors.New("transport: connection refused"), "permission: check", errorx.KindUnavailable)
	esc := KindToCode(wrapped)
	require.NotNil(t, esc)
	assert.Equal(t, http.StatusServiceUnavailable, esc.StatusCode)
	assert.Equal(t, api.ErrorCodeUnavailable, esc.Response.Code)
	assert.Contains(t, esc.Response.Message, "unavailable")
}

func TestKindToCode_Kinds(t *testing.T) {
	cases := []struct {
		kind     errorx.Kind
		wantHTTP int
		wantCode api.ErrorCode
	}{
		{errorx.KindCanceled, 499, api.ErrorCodeCanceled},
		{errorx.KindInvalidArgument, http.StatusBadRequest, api.ErrorCodeInvalidArgument},
		{errorx.KindDeadlineExceeded, http.StatusGatewayTimeout, api.ErrorCodeDeadlineExceeded},
		{errorx.KindNotFound, http.StatusNotFound, api.ErrorCodeNotFound},
		{errorx.KindAlreadyExists, http.StatusConflict, api.ErrorCodeAlreadyExists},
		{errorx.KindPermissionDenied, http.StatusForbidden, api.ErrorCodePermissionDenied},
		{errorx.KindResourceExhausted, http.StatusTooManyRequests, api.ErrorCodeResourceExhausted},
		{errorx.KindFailedPrecondition, http.StatusPreconditionFailed, api.ErrorCodeFailedPrecondition},
		{errorx.KindAborted, http.StatusConflict, api.ErrorCodeAborted},
		{errorx.KindOutOfRange, http.StatusBadRequest, api.ErrorCodeOutOfRange},
		{errorx.KindUnimplemented, http.StatusNotImplemented, api.ErrorCodeUnimplemented},
		{errorx.KindInternal, http.StatusInternalServerError, api.ErrorCodeInternal},
		{errorx.KindUnavailable, http.StatusServiceUnavailable, api.ErrorCodeUnavailable},
		{errorx.KindDataLoss, http.StatusInternalServerError, api.ErrorCodeDataLoss},
		{errorx.KindUnauthenticated, http.StatusUnauthorized, api.ErrorCodeUnauthenticated},
		{errorx.KindRevisionConflict, http.StatusConflict, api.ErrorCodeRevisionConflict},
		{errorx.KindUnknown, http.StatusInternalServerError, api.ErrorCodeInternal},
	}
	for _, c := range cases {
		t.Run(c.kind.String(), func(t *testing.T) {
			esc := KindToCode(errorx.New(c.kind, "x"))
			require.NotNil(t, esc)
			assert.Equal(t, c.wantHTTP, esc.StatusCode)
			assert.Equal(t, c.wantCode, esc.Response.Code)
		})
	}
}

func TestKindToCode_NonErrorxFallsToInternal(t *testing.T) {
	esc := KindToCode(errors.New("plain error"))
	require.NotNil(t, esc)
	assert.Equal(t, http.StatusInternalServerError, esc.StatusCode)
	assert.Equal(t, api.ErrorCodeInternal, esc.Response.Code)
}

func TestErrorMiddleware_NilErrorPropagates(t *testing.T) {
	mw := ErrorMiddleware()
	resp, err := mw(testReq(), func(req middleware.Request) (middleware.Response, error) {
		return middleware.Response{}, nil
	})
	require.NoError(t, err)
	assert.Equal(t, middleware.Response{}, resp)
}

func TestErrorMiddleware_WrapsAsESCInChain(t *testing.T) {
	mw := ErrorMiddleware()
	_, err := mw(testReq(), func(req middleware.Request) (middleware.Response, error) {
		return middleware.Response{}, errorx.New(errorx.KindPermissionDenied, "nope")
	})
	require.Error(t, err)

	var esc *api.ErrorStatusCode
	require.True(t, errors.As(err, &esc), "middleware must wrap ESC in chain for ogen to unwrap")
	assert.Equal(t, http.StatusForbidden, esc.StatusCode)
	assert.Equal(t, api.ErrorCodePermissionDenied, esc.Response.Code)
}

func testReq() middleware.Request {
	return middleware.Request{
		Context:       context.Background(),
		OperationName: "test",
	}
}
