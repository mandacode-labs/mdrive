package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSetCookieHonorsSecureFlag pins the regression fix for the
// /auth/me 401 incident. setCookie used to derive Secure from
// r.TLS != nil, but production sits behind a TLS-terminating
// ingress so r.TLS is always nil on the pod and cookies were set
// without the Secure flag even when the config asked for it. The
// cookie then never reached the API on cross-origin XHR, so
// /auth/me always 401'd after a successful login.
//
// CookieSecure on Service is now wired from http.cookie.secure in
// the config; setCookie uses that value directly.
func TestSetCookieHonorsSecureFlag(t *testing.T) {
	cases := []struct {
		name           string
		cookieSecure   bool
		wantSecureAttr bool
	}{
		{"config true => Secure set", true, true},
		{"config false => Secure omitted", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &Service{
				cookieName:     "mdrive_session",
				cookieDomain:   "mdrive.mandacode.com",
				cookieSameSite: http.SameSiteLaxMode,
				cookieSecure:   tc.cookieSecure,
			}

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			// r.TLS is nil on the pod in production; setCookie
			// must not consult it.
			assert.Nil(t, r.TLS)

			svc.setCookie(w, r, "mdrive_session", "v", 60)

			cookies := w.Result().Cookies()
			if assert.Len(t, cookies, 1, "exactly one Set-Cookie expected") {
				assert.Equal(t, tc.wantSecureAttr, cookies[0].Secure,
					"Secure flag must follow cfg.HTTP.Cookie.Secure, not r.TLS")
				assert.Equal(t, "mdrive.mandacode.com", cookies[0].Domain)
				assert.Equal(t, http.SameSiteLaxMode, cookies[0].SameSite)
				assert.True(t, cookies[0].HttpOnly)
			}
		})
	}
}

// TestSetCookieSameAttributesForStateAndSession makes sure the
// state cookie and the session cookie both flow through the same
// setCookie and so inherit the same Secure behavior. The state
// cookie is what /auth/login sets on a top-level redirect; the
// session cookie is what /auth/callback sets just before the
// post-login redirect. Both must have Secure aligned with config.
func TestSetCookieSameAttributesForStateAndSession(t *testing.T) {
	svc := &Service{
		cookieName:     "mdrive_session",
		cookieDomain:   "mdrive.mandacode.com",
		cookieSameSite: http.SameSiteLaxMode,
		cookieSecure:   true,
	}
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	w1 := httptest.NewRecorder()
	svc.setCookie(w1, r, "auth_state", "x", 600)
	w2 := httptest.NewRecorder()
	svc.setCookie(w2, r, "mdrive_session", "y", 86400)

	state := w1.Result().Cookies()
	sess := w2.Result().Cookies()
	if assert.Len(t, state, 1) && assert.Len(t, sess, 1) {
		assert.Equal(t, state[0].Secure, sess[0].Secure,
			"state and session cookies must agree on Secure")
		assert.True(t, sess[0].Secure)
	}
}
