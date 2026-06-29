package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMigrateDeprecatedAuth(t *testing.T) {
	tests := []struct {
		name            string
		redirectURI     string
		frontendURL     string
		expectedAfter   string
		expectUnchanged bool
	}{
		{
			name:          "both empty leaves empty",
			redirectURI:   "",
			frontendURL:   "",
			expectedAfter: "",
		},
		{
			name:          "redirect_uri only is preserved",
			redirectURI:   "https://app.example.com/auth/callback",
			frontendURL:   "",
			expectedAfter: "https://app.example.com/auth/callback",
		},
		{
			name:          "frontend_url only derives redirect_uri",
			redirectURI:   "",
			frontendURL:   "https://app.example.com",
			expectedAfter: "https://app.example.com/auth/callback",
		},
		{
			name:          "both set: redirect_uri wins",
			redirectURI:   "https://canonical.example.com/auth/callback",
			frontendURL:   "https://app.example.com",
			expectedAfter: "https://canonical.example.com/auth/callback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{
				Auth: AuthConfig{
					RedirectURI: tt.redirectURI,
					FrontendURL: tt.frontendURL,
				},
			}
			c.MigrateDeprecatedAuth()
			assert.Equal(t, tt.expectedAfter, c.Auth.RedirectURI)
		})
	}
}

func TestMigrateDeprecatedAuthPreservesNonAuthFields(t *testing.T) {
	c := &Config{
		Auth: AuthConfig{
			RedirectURI: "https://app.example.com/auth/callback",
			FrontendURL: "https://app.example.com",
		},
		Crypto: CryptoConfig{MasterKey: "secret"},
	}
	originalKey := c.Crypto.MasterKey
	c.MigrateDeprecatedAuth()
	assert.Equal(t, "secret", c.Crypto.MasterKey)
	assert.Equal(t, originalKey, c.Crypto.MasterKey)
}