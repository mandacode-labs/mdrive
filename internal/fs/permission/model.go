// Package permission provides authorization primitives backed by OpenFGA.
package permission

import (
	_ "embed"
)

//go:embed model.fga
var ModelDSL string

//go:embed model.json
var ModelJSON []byte
