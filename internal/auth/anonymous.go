package auth

import (
	"encoding/json"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// anonymousPaths returns the set of URL paths whose OpenAPI operation
// declares `security: []`, opting out of the global bearerAuth
// requirement. AuthBridge looks paths up in this set at request time
// so the OpenAPI spec is the single source of truth for which paths
// are public -- adding a new public endpoint only requires editing
// the spec, not this package.
//
// A nil `security` field on an operation means it inherits the
// global `security`, which is bearerAuth; those paths are not
// returned. Only an explicitly empty `security: []` is treated as
// public.
//
// HTTP method entries (get, post, ...) coexist with extension
// fields like x-ogen-operation-group on the same path object, so
// operations are decoded as RawMessage first and only those that
// parse as objects are inspected for `security`.
func anonymousPaths(spec []byte) (map[string]bool, error) {
	var s struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(spec, &s); err != nil {
		return nil, errorx.Wrap(err, "auth: parse openapi spec (bytes=%d)", len(spec))
	}
	out := make(map[string]bool)
	for path, methods := range s.Paths {
		for _, raw := range methods {
			var op struct {
				Security *[]json.RawMessage `json:"security"`
			}
			if !json.Valid(raw) {
				continue
			}
			if err := json.Unmarshal(raw, &op); err != nil {
				continue
			}
			if op.Security == nil {
				continue
			}
			if len(*op.Security) == 0 {
				out[path] = true
			}
		}
	}
	return out, nil
}
