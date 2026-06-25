// Package apiopts provides small typed-option helpers used by
// the API handler to convert between Go pointer types and the
// ogen-generated *OptT wrappers. It intentionally has no domain
// dependencies so it cannot become a dumping ground.
package apiopts

import "github.com/mandacode-labs/mdrive/pkg/api"

// OptString returns an api.OptString with the value set.
func OptString(s string) api.OptString {
	return api.OptString{Value: s, Set: true}
}

// OptStringPtr returns an api.OptString from a pointer value.
func OptStringPtr(s *string) api.OptString {
	if s == nil {
		return api.OptString{}
	}
	return api.OptString{Value: *s, Set: true}
}

// OptBool returns an api.OptBool with the value set.
func OptBool(b bool) api.OptBool {
	return api.OptBool{Value: b, Set: true}
}