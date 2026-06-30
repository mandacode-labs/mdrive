package auth

import (
	"github.com/coreos/go-oidc/v3/oidc"
)

// keycloakClaims is the subset of Keycloak's id_token claims we
// read. Realm roles are nested under `realm_access.roles` per
// Keycloak's OIDC token structure.
type keycloakClaims struct {
	RealmAccess struct {
		Roles []string `json:"roles"`
	} `json:"realm_access"`
}

// isAdminRole returns true when the id_token's `realm_access.roles`
// contains the configured admin role name. Per OIDC Core, the
// id_token's Claims() method only succeeds for claims the verifier
// permits — but Keycloak emits realm_access.roles in the default
// mapper, so this works in practice.
func isAdminRole(idToken *oidc.IDToken) bool {
	var c keycloakClaims
	if err := idToken.Claims(&c); err != nil {
		return false
	}
	for _, r := range c.RealmAccess.Roles {
		if r == AdminRole {
			return true
		}
	}
	return false
}
