package permission

import (
	"strings"
	"testing"
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
			},
			mode: AuthModeClientCredentials,
		},
		{
			name:    "client_credentials_missing_secret",
			cfg:     Config{ClientID: "id", TokenIssuer: "x", Audience: "y"},
			mode:    AuthModeClientCredentials,
			wantErr: true,
			errSub:  "requires client_id and client_secret",
		},
		{
			name:    "client_credentials_missing_issuer",
			cfg:     Config{ClientID: "id", ClientSecret: "s", Audience: "a"},
			mode:    AuthModeClientCredentials,
			wantErr: true,
			errSub:  "requires token_issuer and audience",
		},
		{
			name:    "client_credentials_with_api_token_rejected",
			cfg:     Config{APIToken: "x", ClientID: "y", ClientSecret: "z", TokenIssuer: "i", Audience: "a"},
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
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errSub)
				}
				if tt.errSub != "" && !strings.Contains(err.Error(), tt.errSub) {
					t.Fatalf("expected error containing %q, got %q", tt.errSub, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.mode == AuthModeNone {
				if creds != nil {
					t.Fatalf("auth_mode=none should produce nil credentials, got %v", creds)
				}
			} else if creds == nil {
				t.Fatalf("expected non-nil credentials for mode %s", tt.mode)
			}
		})
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || strings.Contains(s, sub)
}
