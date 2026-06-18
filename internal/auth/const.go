package auth

const (
	// SessionCookieName is the name of the HTTP cookie used for web sessions.
	SessionCookieName = "mdrive_sid"

	// PKCEPrefix is the key prefix used to store PKCE code verifiers in the session store.
	PKCEPrefix = "pkce:"

	// DefaultOIDCScopes are the OIDC scopes requested during authorization.
	DefaultOIDCScopes = "openid profile email"

	// AdminRoleClaim is the Zitadel project role claim key used to identify admins.
	AdminRoleClaim = "urn:zitadel:iam:org:project:roles"

	// AdminRole is the role name that grants admin access.
	AdminRole = "admin"

	scopeOpenID  = "openid"
	scopeProfile = "profile"
	scopeEmail   = "email"
)
