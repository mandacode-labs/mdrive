package permission

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildCredentials(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		mode    AuthMode
		wantErr bool
		errSub  string
	}{
		{
			name: "api_token_ok",
			cfg:  Config{APIToken: "secret"},
			mode: AuthModeAPIToken,
		},
		{
			name:    "api_token_missing",
			cfg:     Config{},
			mode:    AuthModeAPIToken,
			wantErr: true,
			errSub:  "requires api_token",
		},
		{
			name:    "api_token_with_client_rejected",
			cfg:     Config{APIToken: "x", ClientID: "y"},
			mode:    AuthModeAPIToken,
			wantErr: true,
			errSub:  "does not accept client_id",
		},
		{
			name: "client_credentials_ok",
			cfg: Config{
				ClientID:     "id",
				ClientSecret: "secret",
				TokenIssuer:  "https://issuer",
				Audience:     "api",
				Scopes:       []string{"openfga-api"},
			},
			mode: AuthModeClientCredentials,
		},
		{
			name: "client_credentials_multi_scopes",
			cfg: Config{
				ClientID:     "id",
				ClientSecret: "secret",
				TokenIssuer:  "https://issuer",
				Audience:     "api",
				Scopes:       []string{"openfga-api", "aud-2"},
			},
			mode: AuthModeClientCredentials,
		},
		{
			name: "client_credentials_empty_scopes_allowed",
			cfg:  Config{ClientID: "id", ClientSecret: "s", TokenIssuer: "i", Audience: "a"},
			mode: AuthModeClientCredentials,
		},
		{
			name:    "client_credentials_missing_secret",
			cfg:     Config{ClientID: "id", TokenIssuer: "x", Audience: "y", Scopes: []string{"openfga-api"}},
			mode:    AuthModeClientCredentials,
			wantErr: true,
			errSub:  "requires client_id and client_secret",
		},
		{
			name:    "client_credentials_missing_issuer",
			cfg:     Config{ClientID: "id", ClientSecret: "s", Audience: "a", Scopes: []string{"openfga-api"}},
			mode:    AuthModeClientCredentials,
			wantErr: true,
			errSub:  "requires token_issuer and audience",
		},
		{
			name:    "client_credentials_with_api_token_rejected",
			cfg:     Config{APIToken: "x", ClientID: "y", ClientSecret: "z", TokenIssuer: "i", Audience: "a", Scopes: []string{"openfga-api"}},
			mode:    AuthModeClientCredentials,
			wantErr: true,
			errSub:  "does not accept api_token",
		},
		{
			name: "none_ok",
			cfg:  Config{},
			mode: AuthModeNone,
		},
		{
			name:    "none_with_api_token_rejected",
			cfg:     Config{APIToken: "x"},
			mode:    AuthModeNone,
			wantErr: true,
			errSub:  "does not accept any credentials",
		},
		{
			name:    "unknown_mode",
			cfg:     Config{},
			mode:    AuthMode("magic"),
			wantErr: true,
			errSub:  "invalid openfga auth_mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds, err := buildCredentials(tt.cfg, tt.mode)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errSub != "" {
					assert.Contains(t, err.Error(), tt.errSub)
				}
				return
			}
			require.NoError(t, err)
			if tt.mode == AuthModeNone {
				assert.Nil(t, creds, "auth_mode=none should produce nil credentials")
			} else {
				assert.NotNil(t, creds, "expected non-nil credentials for mode %s", tt.mode)
			}
		})
	}
}
