// Package vfs is the service layer for mdrive.
//
// It orchestrates node, drive, and user domain operations with cross-cutting
// concerns: S3 interaction (via GarbageRecorder) and transactional integrity
// (via node.Repository.WithTx). Permission checks are the caller's
// responsibility — vfs itself only does path resolution.
//
// The core packages (internal/core/node, drive, user) are pure domain types
// and data-access contracts. vfs is the first layer that composes them.
package vfs
