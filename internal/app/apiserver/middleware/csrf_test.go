package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCSRFMiddleware(t *testing.T) {
	cfg := CSRFConfig{AllowedOrigins: []string{"https://app.example.com"}}
	wrap := CSRFMiddleware(cfg)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := wrap(next)

	tests := []struct {
		name       string
		method     string
		origin     string
		referer    string
		wantStatus int
	}{
		{
			name:       "POST with allowed Origin passes",
			method:     http.MethodPost,
			origin:     "https://app.example.com",
			wantStatus: http.StatusOK,
		},
		{
			name:       "POST with disallowed Origin is rejected",
			method:     http.MethodPost,
			origin:     "https://evil.example.com",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "POST with Referer fallback (allowed) passes",
			method:     http.MethodPut,
			referer:    "https://app.example.com/path",
			wantStatus: http.StatusOK,
		},
		{
			name:       "POST with Referer fallback (disallowed) rejected",
			method:     http.MethodPut,
			referer:    "https://evil.example.com/path",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "POST with neither Origin nor Referer rejected",
			method:     http.MethodPost,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "GET is exempt",
			method:     http.MethodGet,
			wantStatus: http.StatusOK,
		},
		{
			name:       "OPTIONS is exempt",
			method:     http.MethodOptions,
			wantStatus: http.StatusOK,
		},
		{
			name:       "Origin comparison is case-insensitive on scheme and host",
			method:     http.MethodPost,
			origin:     "HTTPS://APP.EXAMPLE.COM",
			wantStatus: http.StatusOK,
		},
		{
			name:       "Origin with port is matched exactly",
			method:     http.MethodPost,
			origin:     "https://app.example.com:8443",
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(tt.method, "/anything", nil)
			if tt.origin != "" {
				r.Header.Set("Origin", tt.origin)
			}
			if tt.referer != "" {
				r.Header.Set("Referer", tt.referer)
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != tt.wantStatus {
				t.Fatalf("got %d, want %d (body=%s)", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}
