// Package perm provides authorization primitives backed by OpenFGA.
package perm

import (
	_ "embed"
)

//go:embed model.fga
var ModelDSL string

//go:embed model.json
var ModelJSON []byte
