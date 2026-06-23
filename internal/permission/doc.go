// Package permission is the access-control layer. It wraps an
// OpenFGA client and exposes a typed Checker interface
// (Permission enum, ObjectTypeDrive constant) plus a Require
// helper that returns the single permission.ErrPermission sentinel.
package permission
