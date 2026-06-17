# Go Convention Audit & Fix Plan

## Completed Fixes

### Critical — Error Ignoring (`_`)
| # | File | Fix |
|---|------|-----|
| 1 | `apiserver/error.go:51,57` | `json.Encode` errors handled gracefully |
| 2 | `handler/auth.go:22` | `generatePKCE()` error propagated |
| 3 | `handler/auth.go:68` | `GetPKCE()` error propagated (PKCE expiry) |
| 4 | `auth/service.go:71-72` | `id_token` type assertion checked, verification error propagated |
| 5 | `auth/service.go:97-98` | Same fix for `ExchangeCode` |
| 6 | `auth/service.go:163` | PKCE cleanup: best-effort with comment |
| 7 | `auth/security.go:64` | Context type assertion `ok` checked |
| 8 | `config/config.go:109` | `ParseDuration` error handled |

### Interface Naming
| # | Fix |
|---|-----|
| 9 | `GCClient` → `TombstoneInserter` (single-method = `-er` suffix) |

### Modern Go
| # | Fix |
|---|-----|
| 10 | `interface{}` → `any` (3 test files) |
| 11 | `[]DirEntry{}` → removed (nil param already handled) |

### Dead Code
| # | Fix |
|---|-----|
| 12 | `var _ = oauth2.Token{}` + unused `oauth2` import removed |
| 13 | `var _ = fmt.Sprintf` + `fmt` import removed |
| 14 | `var now = time.Now` + unused `time` import removed |
| 15 | Broken `itoa()` → `strconv.Itoa` |

### Magic Strings → Constants
| # | Fix |
|---|-----|
| 16 | `"application/json"` → `contentTypeJSON` |
| 17 | `"google"` → `providerGoogle` |
| 18 | `"mdrive_sid"` → `auth.SessionCookieName` |
| 19 | `"pkce:"` → `auth.PKCEPrefix` |
| 20 | `"openid profile email"` → `auth.DefaultOIDCScopes` |
| 21 | `"mdrive:session:"` → `session.keyPrefix` |
| 22 | `"mdrive:upload:"` → `upload.DefaultKeyPrefix` |

## Verified Compliant (no changes needed)

| Rule | Status |
|------|--------|
| Initialisms (URL, ID, HTTP, API) | ✅ all correct |
| Import grouping (std → blank → 3rd-party) | ✅ |
| Variable names (short scope = short name) | ✅ |
| Receiver names (1-2 letter, consistent) | ✅ |
| Error strings (lowercase, no punctuation) | ✅ |
| MixedCaps (maxLength, not MaxLength) | ✅ |
| Crypto rand (no math/rand) | ✅ |
| Context as first parameter | ✅ |
| Doc comments on exported types | ✅ |
| Naked returns (none in medium/large funcs) | ✅ |
| No util/common/misc package names | ✅ |
| Dir name = package name | ✅ |
