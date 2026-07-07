// Package nodeop is the ent-backed implementation of
// fs.NodeOperation. It mirrors Linux's inode_operations
// handlers: small per-method files (lookup, create, link,
// unlink, rmdir, rename) plus a single node_repo.go that
// owns the data-access contract.
//
// Permission is the caller's responsibility; nodeop
// enforces structural invariants only.
package nodeop
