package vfs

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/mandacode-labs/mdrive/internal/core/node"
)

// Node operations — delegates to the node domain service.

// CreateFile creates and persists a file node.
func (s *Service) CreateFile(ctx context.Context, content string) (*node.Node, error) {
	n, err := s.nodeSvc.CreateFile(ctx, content)
	if err != nil {
		return nil, fmt.Errorf("vfs: %w", err)
	}
	return n, nil
}

// CreateDirectory creates and persists a directory node.
func (s *Service) CreateDirectory(ctx context.Context) (*node.Node, error) {
	n, err := s.nodeSvc.CreateDirectory(ctx)
	if err != nil {
		return nil, fmt.Errorf("vfs: %w", err)
	}
	return n, nil
}

// CreateSymlink creates and persists a symlink node.
func (s *Service) CreateSymlink(ctx context.Context, target string) (*node.Node, error) {
	n, err := s.nodeSvc.CreateSymlink(ctx, target)
	if err != nil {
		return nil, fmt.Errorf("vfs: %w", err)
	}
	return n, nil
}

// CreateObject creates and persists an object node.
func (s *Service) CreateObject(ctx context.Context, content node.ObjectContent, size int64) (*node.Node, error) {
	n, err := s.nodeSvc.CreateObject(ctx, content, size)
	if err != nil {
		return nil, fmt.Errorf("vfs: %w", err)
	}
	return n, nil
}

// Link adds a child entry to parent and persists the parent.
func (s *Service) Link(ctx context.Context, parent *node.Node, name string, child *node.Node) error {
	return s.nodeSvc.Link(ctx, parent, name, child)
}

// Unlink removes a child entry from parent and persists the parent.
func (s *Service) Unlink(ctx context.Context, parent *node.Node, name string) error {
	return s.nodeSvc.Unlink(ctx, parent, name)
}

// GetNode returns a node by ID.
func (s *Service) GetNode(ctx context.Context, id uuid.UUID) (*node.Node, error) {
	return s.nodeSvc.GetByID(ctx, id)
}

// UpdateNode saves a node after mutation.
func (s *Service) UpdateNode(ctx context.Context, n *node.Node) error {
	return s.nodeSvc.Update(ctx, n)
}

// DeleteNode removes a node by ID.
func (s *Service) DeleteNode(ctx context.Context, id uuid.UUID) error {
	return s.nodeSvc.Delete(ctx, id)
}

// ReadObjectData fetches S3 data for an object node.
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

// ObjectExists checks whether the S3 object backing an object node exists.
func (s *Service) ObjectExists(ctx context.Context, n *node.Node) (bool, error) {
	oc, err := n.ReadObject()
	if err != nil {
		return false, err
	}
	return s.store.ObjectExists(ctx, oc.Bucket, oc.Key)
}
