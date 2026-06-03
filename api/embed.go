package openapispec

import _ "embed"

//go:embed openapi.bundled.json
var Spec []byte
