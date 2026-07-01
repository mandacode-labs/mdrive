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

func TestWrapInheritsKindFromSentinel(t *testing.T) {
	sentinel := New(KindBadRequest, "bad input")
	wrapped := Wrap(sentinel, "while parsing field X")
	assert.Equal(t, KindBadRequest, wrapped.Kind(),
		"Wrap must inherit Kind from a wrapped errorx.Error")
	assert.Equal(t, "while parsing field X", wrapped.Error())
}

func TestWrapFallsBackToUnknownForPlainError(t *testing.T) {
	wrapped := Wrap(errors.New("plain"), "context")
	assert.Equal(t, KindUnknown, wrapped.Kind())
}

func TestWrapUnwrapsCause(t *testing.T) {
	cause := errors.New("root cause")
	wrapped := Wrap(cause, "context")
	assert.Equal(t, cause, errors.Unwrap(wrapped),
		"Wrap must preserve the original error via errors.Unwrap")
}

func TestWrapNilReturnsNil(t *testing.T) {
	assert.Nil(t, Wrap(nil, "msg"))
}

func TestWrapFormatsArgs(t *testing.T) {
	wrapped := Wrap(New(KindNotFound, "missing"), "field %s of %d", "name", 7)
	assert.Equal(t, "field name of 7", wrapped.Error())
}

func TestErrorsAsRecognizesWrapped(t *testing.T) {
	wrapped := Wrap(errors.New("plain"), "msg")
	var de Error
	assert.True(t, errors.As(wrapped, &de))
	assert.Equal(t, KindUnknown, de.Kind())
}

func TestKindOfTraversesChain(t *testing.T) {
	leaf := New(KindServiceDegraded, "downstream broken")
	mid := Wrap(leaf, "while sending event")
	outer := Wrap(mid, "during request")
	assert.Equal(t, KindServiceDegraded, KindOf(outer))
}

func TestKindOfReturnsUnknownForPlainChain(t *testing.T) {
	plain := errors.New("plain")
	wrapped := Wrap(plain, "context")
	assert.Equal(t, KindUnknown, KindOf(wrapped))
}

func TestKindOfNilReturnsUnknown(t *testing.T) {
	assert.Equal(t, KindUnknown, KindOf(nil))
}

func TestChainRendersOuterToInner(t *testing.T) {
	cause := errors.New("cause")
	wrapped := Wrap(cause, "outer")
	assert.Equal(t, "outer -> cause", Chain(wrapped))
}

func TestChainEmptyForNil(t *testing.T) {
	assert.Equal(t, "", Chain(nil))
}

func TestChainSingleForUnwrapped(t *testing.T) {
	assert.Equal(t, "only", Chain(errors.New("only")))
}

func TestWrapSentinelKeepsKind(t *testing.T) {
	sentinel := New(KindForbidden, "vfs: cycle detected")
	err := errors.New("cycle: a -> b -> a")
	wrapped := WrapSentinel(sentinel, err, "vfs: mount cycle check")

	assert.Equal(t, KindForbidden, wrapped.Kind(),
		"WrapSentinel must surface the sentinel's kind even when err is a plain error")
	assert.True(t, errors.Is(wrapped, sentinel),
		"errors.Is must walk the chain and find the sentinel")
}

func TestWrapSentinelWithoutErrStillMatches(t *testing.T) {
	sentinel := New(KindBadRequest, "vfs: no parent")
	wrapped := WrapSentinel(sentinel, nil, "vfs: validation")

	assert.Equal(t, KindBadRequest, wrapped.Kind())
	assert.True(t, errors.Is(wrapped, sentinel),
		"WrapSentinel(err=nil) must still expose the sentinel for errors.Is")
}
