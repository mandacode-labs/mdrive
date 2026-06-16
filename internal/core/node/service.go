package node

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// Service is the node-domain service. It encapsulates node-level operations
// (create, link, unlink) using the Repository for persistence.
//
// The Service has no dependencies on storage or permission subsystems;
// orchestration across multiple concerns lives in higher layers (e.g., application/vfs).
type Service struct {
	repo Repository
}

// NewService creates a new Service backed by the given Repository.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// CreateFile creates a new file node and persists it.
func (s *Service) CreateFile(ctx context.Context, content string) (*Node, error) {
	n, err := NewFile(content)
	if err != nil {
		return nil, fmt.Errorf("create file: %w", err)
	}
	if err := s.repo.Save(ctx, n); err != nil {
		return nil, fmt.Errorf("save file: %w", err)
	}
	return n, nil
}

// CreateDirectory creates a new empty directory node and persists it.
func (s *Service) CreateDirectory(ctx context.Context) (*Node, error) {
	n, err := NewDirectory()
	if err != nil {
		return nil, fmt.Errorf("create directory: %w", err)
	}
	if err := s.repo.Save(ctx, n); err != nil {
		return nil, fmt.Errorf("save directory: %w", err)
	}
	return n, nil
}

// CreateSymlink creates a new symlink node and persists it.
func (s *Service) CreateSymlink(ctx context.Context, target string) (*Node, error) {
	n, err := NewSymlink(target)
	if err != nil {
		return nil, fmt.Errorf("create symlink: %w", err)
	}
	if err := s.repo.Save(ctx, n); err != nil {
		return nil, fmt.Errorf("save symlink: %w", err)
	}
	return n, nil
}

// CreateObject creates a new object node and persists it.
func (s *Service) CreateObject(ctx context.Context, content ObjectContent, size int64) (*Node, error) {
	n, err := NewObject(content, size)
	if err != nil {
		return nil, fmt.Errorf("create object: %w", err)
	}
	if err := s.repo.Save(ctx, n); err != nil {
		return nil, fmt.Errorf("save object: %w", err)
	}
	return n, nil
}

// Link adds a child to a parent directory and persists the updated parent.
func (s *Service) Link(ctx context.Context, parent *Node, name string, child *Node) error {
	if parent == nil {
		return errors.New("link: parent is nil")
	}
	if child == nil {
		return errors.New("link: child is nil")
	}
	if err := parent.AddEntry(name, child); err != nil {
		return fmt.Errorf("link: %w", err)
	}
	if err := s.repo.Save(ctx, parent); err != nil {
		return fmt.Errorf("link: save parent: %w", err)
	}
	return nil
}

// Unlink removes a child entry from a parent directory and persists the updated parent.
func (s *Service) Unlink(ctx context.Context, parent *Node, name string) error {
	if parent == nil {
		return errors.New("unlink: parent is nil")
	}
	if err := parent.RemoveEntry(name); err != nil {
		return fmt.Errorf("unlink: %w", err)
	}
	if err := s.repo.Save(ctx, parent); err != nil {
		return fmt.Errorf("unlink: save parent: %w", err)
	}
	return nil
}

// GetByID returns the node with the given id.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*Node, error) {
	n, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get node: %w", err)
	}
	return n, nil
}

// Update persists the node after its content has been mutated
// (e.g., via WriteFile, WriteDir, WriteSymlink, WriteObject).
func (s *Service) Update(ctx context.Context, n *Node) error {
	if n == nil {
		return errors.New("update: node is nil")
	}
	if err := s.repo.Save(ctx, n); err != nil {
		return fmt.Errorf("update: %w", err)
	}
	return nil
}

// Delete removes the node with the given id (hard delete).
// For Object nodes, callers should delete the S3 object first, then call Delete.
// Orphan S3 objects (where the node was deleted but S3 delete failed) are handled by GC.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	return nil
}

// WithTx executes fn within a transaction, exposing a tx-scoped Service.
func (s *Service) WithTx(ctx context.Context, fn func(*Service) error) error {
	return s.repo.WithTx(ctx, func(txRepo Repository) error {
		return fn(&Service{repo: txRepo})
	})
}
