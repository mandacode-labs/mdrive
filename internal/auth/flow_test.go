package auth

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

func newTestKey(t *testing.T) []byte {
	t.Helper()
	b := make([]byte, 32)
	_, err := rand.Read(b)
	require.NoError(t, err)
	return b
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	return &Service{
		encKey:         newTestKey(t),
		cookieName:     "mdrive_session",
		cookieSameSite: 0,
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := newTestKey(t)
	plain := []byte(`{"hello":"world","n":42}`)

	enc, err := encrypt(plain, key)
	require.NoError(t, err)
	assert.NotEmpty(t, enc)

	got, err := decrypt(enc, key)
	require.NoError(t, err)
	assert.Equal(t, plain, got)
}

func TestDecryptRejectsTamperedCiphertext(t *testing.T) {
	key := newTestKey(t)
	enc, err := encrypt([]byte("secret"), key)
	require.NoError(t, err)

	tampered := enc[:len(enc)-2] + "AA"
	_, err = decrypt(tampered, key)
	assert.Error(t, err, "AES-GCM must reject any ciphertext tamper")
}

func TestDecryptRejectsWrongKey(t *testing.T) {
	key1 := newTestKey(t)
	key2 := newTestKey(t)
	enc, err := encrypt([]byte("secret"), key1)
	require.NoError(t, err)

	_, err = decrypt(enc, key2)
	assert.Error(t, err, "decryption with a different key must fail")
}

func TestNonceUniqueness(t *testing.T) {
	seen := make(map[string]bool, 1000)
	for i := 0; i < 1000; i++ {
		n := nonce()
		require.NotEmpty(t, n)
		require.False(t, seen[n], "nonce collided at iteration %d", i)
		seen[n] = true
	}
}

func TestConsumeStateCookieRejectsMissingCookie(t *testing.T) {
	svc := newTestService(t)
	r := httptest.NewRequest(http.MethodGet, "/auth/callback", nil)
	w := httptest.NewRecorder()

	_, err := svc.consumeStateCookie(w, r, "any-state")
	require.Error(t, err, "missing state cookie must error")
	assert.True(t, errors.Is(err, errStateMissing),
		"missing cookie must wrap errStateMissing sentinel")
}

func TestConsumeStateCookieRejectsDecryptFailure(t *testing.T) {
	svc := newTestService(t)
	r := httptest.NewRequest(http.MethodGet, "/auth/callback", nil)
	r.AddCookie(&http.Cookie{Name: "auth_state", Value: "not-valid-base64-!!"})
	w := httptest.NewRecorder()

	_, err := svc.consumeStateCookie(w, r, "any-state")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errStateDecrypt),
		"decrypt failure must wrap errStateDecrypt sentinel")
}

func TestConsumeStateCookieRejectsCorruptJSON(t *testing.T) {
	svc := newTestService(t)
	raw := []byte(`{"this is not valid json`)
	enc, err := encrypt(raw, svc.encKey)
	require.NoError(t, err)

	r := httptest.NewRequest(http.MethodGet, "/auth/callback", nil)
	r.AddCookie(&http.Cookie{Name: "auth_state", Value: enc})
	w := httptest.NewRecorder()

	_, err = svc.consumeStateCookie(w, r, "any-state")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errStateCorrupt),
		"corrupt payload must wrap errStateCorrupt sentinel")
}

func TestConsumeStateCookieRejectsMismatchedState(t *testing.T) {
	svc := newTestService(t)
	sd := stateData{State: "real-state", Verifier: "v"}
	raw, err := json.Marshal(sd)
	require.NoError(t, err)
	enc, err := encrypt(raw, svc.encKey)
	require.NoError(t, err)

	r := httptest.NewRequest(http.MethodGet, "/auth/callback?state=attacker-state", nil)
	r.AddCookie(&http.Cookie{Name: "auth_state", Value: enc})
	w := httptest.NewRecorder()

	_, err = svc.consumeStateCookie(w, r, "attacker-state")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errStateMismatch),
		"state mismatch must wrap errStateMismatch sentinel")

	cleared := w.Header().Get("Set-Cookie")
	assert.Contains(t, cleared, "auth_state=", "cookie must be cleared even on rejection")
	assert.Contains(t, cleared, "Max-Age=0", "cookie must be expired")
}

func TestConsumeStateCookieClearsCookieOnSuccess(t *testing.T) {
	svc := newTestService(t)
	sd := stateData{State: "matching", Verifier: "v"}
	raw, err := json.Marshal(sd)
	require.NoError(t, err)
	enc, err := encrypt(raw, svc.encKey)
	require.NoError(t, err)

	r := httptest.NewRequest(http.MethodGet, "/auth/callback?state=matching", nil)
	r.AddCookie(&http.Cookie{Name: "auth_state", Value: enc})
	w := httptest.NewRecorder()

	got, err := svc.consumeStateCookie(w, r, "matching")
	require.NoError(t, err)
	assert.Equal(t, "matching", got.State)
	assert.Equal(t, "v", got.Verifier)

	cleared := w.Header().Get("Set-Cookie")
	assert.Contains(t, cleared, "Max-Age=0", "state cookie must be one-time use")
}

// All four state-failure sentinels must map to 400 via the
// errorx.KindBadRequest contract. This is the "consistent
// handling" guarantee -- one Kind, four messages.
func TestStateSentinelsAllMapToBadRequest(t *testing.T) {
	sentinels := []error{
		errStateMissing,
		errStateDecrypt,
		errStateCorrupt,
		errStateMismatch,
	}
	for _, e := range sentinels {
		var de errorx.Error
		require.True(t, errors.As(e, &de),
			"sentinel %v must satisfy errorx.Error", e)
		assert.Equal(t, errorx.KindBadRequest, de.Kind(),
			"sentinel %v must be KindBadRequest", e)
		assert.Equal(t, http.StatusBadRequest, statusForError(e),
			"sentinel %v must produce 400", e)
	}
}