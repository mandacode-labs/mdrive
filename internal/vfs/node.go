package vfs

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/mandacode-labs/mdrive/internal/core/node"
)

// Node operations on the vfs Service.
//
// These methods compose node domain constructors with repository persistence
// and (where appropriate) permission checks. They are the VFS-level API
// analogous to the Linux VFS layer: path-based operations with metadata
// and error handling.

// CreateFile creates a file node and persists it.
func (s *Service) CreateFile(ctx context.Context, content string) (*node.Node, error) {
	n, err := node.NewFile(content)
	if err != nil {
		return nil, fmt.Errorf("vfs: create file: %w", err)
	}
	if err := s.node.Save(ctx, n); err != nil {
		return nil, fmt.Errorf("vfs: save file: %w", err)
	}
	return n, nil
}

// CreateDirectory creates a directory node and persists it.
func (s *Service) CreateDirectory(ctx context.Context) (*node.Node, error) {
	n, err := node.NewDirectory()
	if err != nil {
		return nil, fmt.Errorf("vfs: create directory: %w", err)
	}
	if err := s.node.Save(ctx, n); err != nil {
		return nil, fmt.Errorf("vfs: save directory: %w", err)
	}
	return n, nil
}

// CreateSymlink creates a symlink node and persists it.
func (s *Service) CreateSymlink(ctx context.Context, target string) (*node.Node, error) {
	n, err := node.NewSymlink(target)
	if err != nil {
		return nil, fmt.Errorf("vfs: create symlink: %w", err)
	}
	if err := s.node.Save(ctx, n); err != nil {
		return nil, fmt.Errorf("vfs: save symlink: %w", err)
	}
	return n, nil
}

// CreateObject creates an object node and persists it.
func (s *Service) CreateObject(ctx context.Context, content node.ObjectContent, size int64) (*node.Node, error) {
	n, err := node.NewObject(content, size)
	if err != nil {
		return nil, fmt.Errorf("vfs: create object: %w", err)
	}
	if err := s.node.Save(ctx, n); err != nil {
		return nil, fmt.Errorf("vfs: save object: %w", err)
	}
	return n, nil
}

// Link adds a child entry to a parent directory and persists the parent.
func (s *Service) Link(ctx context.Context, parent *node.Node, name string, child *node.Node) error {
	if err := parent.AddEntry(name, child); err != nil {
		return fmt.Errorf("vfs: link: %w", err)
	}
	return s.node.Save(ctx, parent)
}

// Unlink removes a child entry from a parent directory and persists the parent.
func (s *Service) Unlink(ctx context.Context, parent *node.Node, name string) error {
	if err := parent.RemoveEntry(name); err != nil {
		return fmt.Errorf("vfs: unlink: %w", err)
	}
	return s.node.Save(ctx, parent)
}

// GetNode returns a node by ID.
func (s *Service) GetNode(ctx context.Context, id uuid.UUID) (*node.Node, error) {
	n, err := s.node.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("vfs: get node: %w", err)
	}
	return n, nil
}

// UpdateNode persists changes to a node after its mutable fields have been modified.
func (s *Service) UpdateNode(ctx context.Context, n *node.Node) error {
	return s.node.Save(ctx, n)
}

// DeleteNode removes a node by ID.
func (s *Service) DeleteNode(ctx context.Context, id uuid.UUID) error {
	return s.node.Delete(ctx, id)
}

// ReadObjectData fetches the actual S3 data for an object node.
// This combines the domain (reading ObjectContent) with infrastructure (S3 GET).
func (s *Service) ReadObjectData(ctx context.Context, n *node.Node) ([]byte, error) {
	oc, err := n.ReadObject()
	if err != nil {
		return nil, fmt.Errorf("vfs: read object ref: %w", err)
	}
	data, err := s.store.GetObject(ctx, oc.Bucket, oc.Key)
	if err != nil {
		return nil, fmt.Errorf("vfs: store get: %w", err)
	}
	return data, nil
}

// ObjectExists checks whether the S3 object backing an object node still exists.
func (s *Service) ObjectExists(ctx context.Context, n *node.Node) (bool, error) {
	oc, err := n.ReadObject()
	if err != nil {
		return false, err
	}
	return s.store.ObjectExists(ctx, oc.Bucket, oc.Key)
}
