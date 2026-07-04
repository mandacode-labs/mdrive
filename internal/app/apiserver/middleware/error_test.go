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
	wrapped := errorx.Wrap(errors.New("transport: connection refused"), "permission: check", errorx.KindServiceDegraded)
	esc := KindToCode(wrapped)
	require.NotNil(t, esc)
	assert.Equal(t, http.StatusServiceUnavailable, esc.StatusCode)
	assert.Equal(t, api.ErrorCodeInternal, esc.Response.Code)
	assert.Contains(t, esc.Response.Message, "service_degraded")
}

func TestKindToCode_Kinds(t *testing.T) {
	cases := []struct {
		kind errorx.Kind
		want int
	}{
		{errorx.KindBadRequest, http.StatusBadRequest},
		{errorx.KindUnauthenticated, http.StatusUnauthorized},
		{errorx.KindForbidden, http.StatusForbidden},
		{errorx.KindNotFound, http.StatusNotFound},
		{errorx.KindConflict, http.StatusConflict},
		{errorx.KindServiceDegraded, http.StatusServiceUnavailable},
		{errorx.KindInternal, http.StatusInternalServerError},
		{errorx.KindUnknown, http.StatusInternalServerError},
	}
	for _, c := range cases {
		t.Run(c.kind.String(), func(t *testing.T) {
			esc := KindToCode(errorx.New(c.kind, "x"))
			require.NotNil(t, esc)
			assert.Equal(t, c.want, esc.StatusCode)
			assert.Equal(t, api.ErrorCodeInternal, esc.Response.Code)
		})
	}
}

func TestKindToCode_NonErrorxFallsToInternal(t *testing.T) {
	esc := KindToCode(errors.New("plain error"))
	require.NotNil(t, esc)
	assert.Equal(t, http.StatusInternalServerError, esc.StatusCode)
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
		return middleware.Response{}, errorx.New(errorx.KindForbidden, "nope")
	})
	require.Error(t, err)

	var esc *api.ErrorStatusCode
	require.True(t, errors.As(err, &esc), "middleware must wrap ESC in chain for ogen to unwrap")
	assert.Equal(t, http.StatusForbidden, esc.StatusCode)
}

func testReq() middleware.Request {
	return middleware.Request{
		Context:       context.Background(),
		OperationName: "test",
	}
}
