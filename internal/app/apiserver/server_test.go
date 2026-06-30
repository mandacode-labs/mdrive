package apiserver

import (
	"net/http"
	"net/url"
	"testing"
)

func TestIsAllowedRedirect(t *testing.T) {
	cases := []struct {
		name    string
		target  string
		allowed []string
		want    bool
	}{
		{"empty target", "", []string{"mdrive.mandacode.com"}, false},
		{"https matching host", "https://mdrive.mandacode.com/dashboard", []string{"mdrive.mandacode.com"}, true},
		{"https with port", "https://mdrive.mandacode.com:8443/x", []string{"mdrive.mandacode.com"}, true},
		{"relative path", "/dashboard", []string{"mdrive.mandacode.com"}, true},
		{"disallowed host", "https://evil.com", []string{"mdrive.mandacode.com"}, false},
		{"subdomain not in allowlist", "https://attacker.mdrive.mandacode.com", []string{"mdrive.mandacode.com"}, false},
		{"protocol relative", "//evil.com/path", []string{"mdrive.mandacode.com"}, false},
		{"http scheme rejected", "http://mdrive.mandacode.com", []string{"mdrive.mandacode.com"}, false},
		{"javascript scheme", "javascript:alert(1)", []string{"mdrive.mandacode.com"}, false},
		{"malformed url", "://not a url", []string{"mdrive.mandacode.com"}, false},
		{"case-insensitive host match", "https://MDRIVE.MANDACODE.COM", []string{"mdrive.mandacode.com"}, true},
		{"empty allowlist", "https://mdrive.mandacode.com", []string{}, false},
		{"multiple allowed hosts", "https://staging.mdrive.mandacode.com", []string{"mdrive.mandacode.com", "staging.mdrive.mandacode.com"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isAllowedRedirect(c.target, c.allowed); got != c.want {
				t.Errorf("isAllowedRedirect(%q, %v) = %v, want %v", c.target, c.allowed, got, c.want)
			}
		})
	}
}

func TestResolveRedirectURI(t *testing.T) {
	const fallback = "https://mdrive.mandacode.com/"
	allowed := []string{"mdrive.mandacode.com"}

	cases := []struct {
		name       string
		query      string
		allowed    []string
		wantTarget string
	}{
		{"valid redirect_uri", "redirect_uri=https://mdrive.mandacode.com/dashboard", allowed, "https://mdrive.mandacode.com/dashboard"},
		{"evil host rejected", "redirect_uri=https://evil.com", allowed, fallback},
		{"empty query", "", allowed, fallback},
		{"relative path", "redirect_uri=/x", allowed, "/x"},
		{"http rejected", "redirect_uri=http://mdrive.mandacode.com", allowed, fallback},
		{"javascript rejected", "redirect_uri=javascript:alert(1)", allowed, fallback},
		{"subdomain not in allowlist", "redirect_uri=https://x.mdrive.mandacode.com", allowed, fallback},
		{"protocol relative rejected", "redirect_uri=//evil.com/x", allowed, fallback},
		{"empty allowlist", "redirect_uri=https://mdrive.mandacode.com", []string{}, fallback},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := &http.Request{URL: &url.URL{RawQuery: c.query}}
			if got := resolveRedirectURI(r, fallback, c.allowed); got != c.wantTarget {
				t.Errorf("resolveRedirectURI(%q) = %q, want %q", c.query, got, c.wantTarget)
			}
		})
	}
}
