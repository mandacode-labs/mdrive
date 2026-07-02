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
		KindInternal:        http.StatusInternalServerError,
		KindUnknown:         http.StatusInternalServerError,
	}
	for k, want := range cases {
		assert.Equal(t, want, k.Status(),
			"Kind %s should map to %d", k, want)
	}
}

func TestErrorIncludesCause(t *testing.T) {
	cause := errors.New("connection refused")
	wrapped := Wrap(cause, "auth: token exchange failed")
	assert.Equal(t, "auth: token exchange failed: connection refused", wrapped.Error(),
		"Error() must join cause with ': ' so user-facing surfaces show the full picture")
}

func TestErrorWithoutCauseReturnsMessage(t *testing.T) {
	noCause := New(KindBadRequest, "auth: missing state cookie")
	assert.Equal(t, "auth: missing state cookie", noCause.Error(),
		"sentinel Error() must return its bare message")
}

func TestWrapInheritsKindFromSentinel(t *testing.T) {
	sentinelInChain := New(KindForbidden, "vfs: cycle detected")
	wrapped := Wrap(sentinelInChain, "vfs: mount cycle check")

	assert.Equal(t, KindForbidden, wrapped.Kind(),
		"Wrap must inherit Kind from a wrapped errorx.Error")
	assert.Equal(t, "vfs: mount cycle check: vfs: cycle detected", wrapped.Error())
}

func TestWrapWithKindOverride(t *testing.T) {
	plain := errors.New("disk full")
	// chain has no errorx.Error so Kind would be Unknown, but caller
	// overrides with KindServiceDegraded.
	wrapped := Wrap(plain, "drive: write failed", KindServiceDegraded)
	assert.Equal(t, KindServiceDegraded, wrapped.Kind(),
		"variadic Kind must override chain inheritance")
}

func TestWrapKindWithoutOverride(t *testing.T) {
	plain := errors.New("connection refused")
	wrapped := Wrap(plain, "auth: lookup failed")
	assert.Equal(t, KindUnknown, wrapped.Kind(),
		"plain chain without override must produce KindUnknown")
}

func TestWrapNilReturnsNil(t *testing.T) {
	assert.Nil(t, Wrap(nil, "msg"))
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

func TestUnwrapPreservesCause(t *testing.T) {
	cause := errors.New("root cause")
	wrapped := Wrap(cause, "context")
	assert.Equal(t, cause, errors.Unwrap(wrapped),
		"Wrap must preserve the original error via errors.Unwrap")
}

func TestErrorsAsRecognizesWrapped(t *testing.T) {
	wrapped := Wrap(errors.New("plain"), "msg", KindBadRequest)
	var de Error
	assert.True(t, errors.As(wrapped, &de))
	assert.Equal(t, KindBadRequest, de.Kind())
}

func TestSentinelStillIdentifiableByKind(t *testing.T) {
	// Sentinel identity travels through the chain by errorx.Error
	// type, not by sentinel object identity. Two distinct sentinel
	// instances with the same kind are NOT equal under errors.Is
	// because they're separate objects. Callers who need
	// equivalence should compare via KindOf rather than errors.Is.
	sentinelInChain := New(KindForbidden, "vfs: cycle detected")
	wrapped := Wrap(sentinelInChain, "vfs: mount cycle check", KindForbidden)
	assert.Equal(t, KindForbidden, KindOf(wrapped),
		"kind must be queryable from the chain without importing the sentinel")
}