// Package vfs is the service layer for mdrive.
//
// It orchestrates node, drive, and user domain operations with cross-cutting
// concerns: permission checks (via permission.Checker), S3 interaction
// (via Storage), and transactional integrity (via Repository.WithTx).
//
// The core packages (internal/core/node, drive, user) are pure domain types
// and data-access contracts. vfs is the first layer that composes them.
package vfs
