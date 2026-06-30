package app

import (
	"net/http"
	"testing"
)

func TestParseSameSite(t *testing.T) {
	cases := []struct {
		in   string
		want http.SameSite
	}{
		{"", http.SameSiteLaxMode},
		{"lax", http.SameSiteLaxMode},
		{"Lax", http.SameSiteLaxMode},
		{"LAX", http.SameSiteLaxMode},
		{"strict", http.SameSiteStrictMode},
		{"none", http.SameSiteNoneMode},
		{"unknown", http.SameSiteLaxMode},
	}
	for _, c := range cases {
		if got := parseSameSite(c.in); got != c.want {
			t.Errorf("parseSameSite(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
