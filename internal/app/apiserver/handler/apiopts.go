package handler

import "github.com/mandacode-labs/mdrive/pkg/api"

// optString returns an api.OptString with the value set.
func optString(s string) api.OptString {
	return api.OptString{Value: s, Set: true}
}

// optStringPtr returns an api.OptString from a pointer value.
func optStringPtr(s *string) api.OptString {
	if s == nil {
		return api.OptString{}
	}
	return api.OptString{Value: *s, Set: true}
}

// optBool returns an api.OptBool with the value set.
func optBool(b bool) api.OptBool {
	return api.OptBool{Value: b, Set: true}
}
