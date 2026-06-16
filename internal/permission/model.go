// Package permission provides authorization primitives backed by OpenFGA.
package permission

import _ "embed"

// AuthorizationModel is the OpenFGA authorization model for mdrive.
// Defines drive and user types with role hierarchy and permissions.
//
//go:embed model.fga
var AuthorizationModel string
