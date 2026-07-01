package errorx

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewHasCorrectKind(t *testing.T) {
	err := New(KindNotFound, "user gone")
	assert.Equal(t, "user gone", err.Error())
	assert.Equal(t, KindNotFound, err.Kind())
}

func TestKindStatus(t *testing.T) {
	cases := map[Kind]int{
		KindNotFound:        http.StatusNotFound,
		KindConflict:        http.StatusConflict,
		KindBadRequest:      http.StatusBadRequest,
		KindForbidden:       http.StatusForbidden,
		KindUnauthenticated: http.StatusUnauthorized,
		KindServiceDegraded: http.StatusServiceUnavailable,
		KindUnknown:         http.StatusInternalServerError,
	}
	for k, want := range cases {
		assert.Equal(t, want, k.Status(),
			"Kind %s should map to %d", k, want)
	}
}

func TestWrapPreservesKind(t *testing.T) {
	wrapped := Wrap(New(KindBadRequest, "bad input"), KindServiceDegraded, "override")
	assert.Equal(t, KindBadRequest, wrapped.Kind(),
		"Wrap must inherit Kind from an existing errorx.Error")
}

func TestWrapSetsKindForPlainError(t *testing.T) {
	wrapped := Wrap(errors.New("plain"), KindServiceDegraded, "wrapped")
	assert.Equal(t, KindServiceDegraded, wrapped.Kind())
}

func TestWrapUnwrapsCause(t *testing.T) {
	cause := errors.New("root cause")
	wrapped := Wrap(cause, KindServiceDegraded, "wrapped")
	assert.Equal(t, cause, errors.Unwrap(wrapped),
		"Wrap must preserve the original error via errors.Unwrap")
}

func TestWrapNilReturnsNil(t *testing.T) {
	assert.Nil(t, Wrap(nil, KindServiceDegraded, "msg"))
}

func TestErrorsAsRecognizesWrapped(t *testing.T) {
	wrapped := Wrap(errors.New("plain"), KindBadRequest, "msg")
	var de Error
	assert.True(t, errors.As(wrapped, &de))
	assert.Equal(t, KindBadRequest, de.Kind())
}