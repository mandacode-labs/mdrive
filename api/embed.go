package openapispec

import _ "embed"

// Spec is the bundled OpenAPI 3.1 specification.
// Regenerate with: npx swagger-cli bundle api/rest/v1/openapi.yaml --outfile api/rest/v1/openapi.bundled.json --type json
//
//go:embed rest/v1/openapi.bundled.json
var Spec []byte
