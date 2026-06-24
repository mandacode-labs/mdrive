// Package ent provides real-database integration tests for the
// ent-backed repositories in internal/core/{drive,node}. Unlike
// the handler-level tests in this directory, these tests spin
// up a real Postgres via testcontainers and exercise the
// entRepository round-trip, optimistic-concurrency, and
// transaction paths that the stub-based tests cannot cover.
//
// Build tag: integration_ent. The Makefile target test-integration-ent
// (and the CI step) enables it; running `go test ./...` without
// the tag skips the entire package, so the file does not need
// to be Docker-aware in dev.
//
//go:build integration_ent

package ent
