package auth

import (
	"time"

	"github.com/mandacode-labs/mdrive/internal/core/user"
)

// NewForTest returns a Service with the parts HandleCookieAuth needs
// (encryption key, cookie name, user lookup) but with no OIDC
// discovery wired up. It exists for tests that exercise the cookie
// auth flow without standing up Keycloak.
//
// Calling Authenticate / Callback / Logout on a service returned by
// this function will panic; those paths need a real provider and
// verifier, which only auth.New supplies.
func NewForTest(users user.Service) *Service {
	return &Service{
		encKey:       make([]byte, 32),
		users:        users,
		providerName: "keycloak",
		cookieName:   "mdrive_session",
		sessionTTL:   24 * time.Hour,
	}
}
