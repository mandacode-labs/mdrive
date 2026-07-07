// Package perm is the access-control layer shared by fs and
// drive. Consumers (fs.Service, drive.Service) import perm.Service
// directly and call Check on it.
//
// The OpenFGA impl lives in openfga.go.
package perm