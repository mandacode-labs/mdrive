package auth

import (
	"time"

	"github.com/mandacode-labs/mdrive/api"
)

// NewForTest returns a Service with the parts AuthBridge needs
// (encryption key, cookie name, user lookup, anonymous path set
// populated from the embedded OpenAPI spec) but with no OIDC
// discovery wired up. It exists for tests that exercise the
// cookie-to-bearer middleware without standing up Keycloak.
// Calling Authenticate / Callback / Logout on a service returned
// by this function will panic; those paths need a real provider
// and verifier, which only auth.New supplies.
func NewForTest(users UserUpserter) *Service {
	paths, err := anonymousPaths(api.Spec)
	if err != nil {
		panic("auth.NewForTest: " + err.Error())
	}
	return &Service{
		encKey:       make([]byte, 32),
		users:        users,
		providerName: "keycloak",
		cookieName:   "mdrive_session",
		sessionTTL:   24 * time.Hour,
		noAuthPaths:  paths,
	}
}
