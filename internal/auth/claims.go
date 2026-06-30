package auth

import (
	"context"

	"github.com/coreos/go-oidc/v3/oidc"
)

type keycloakClaims struct {
	RealmAccess struct {
		Roles []string `json:"roles"`
	} `json:"realm_access"`
}

func isAdminRole(_ context.Context, idToken *oidc.IDToken) bool {
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
