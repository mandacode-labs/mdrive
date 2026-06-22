package node

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Service provides domain-level node operations.
// It wraps Repository with validation and convenience, analogous to
// Linux inode_operations (create, link, unlink, lookup) but without
// path resolution or permission checks — those belong to vfs.
type Service struct {
	repo Repository
}

// NewService creates a new Service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// CreateFile creates and persists a file node.
func (s *Service) CreateFile(ctx context.Context, content string) (*Node, error) {
	n, err := NewFile(content)
	if err != nil {
		return nil, fmt.Errorf("node: create file: %w", err)
	}
	if err := s.repo.Save(ctx, n); err != nil {
		return nil, fmt.Errorf("node: save file: %w", err)
	}
	return n, nil
}

// CreateDirectory creates and persists a directory node.
func (s *Service) CreateDirectory(ctx context.Context) (*Node, error) {
	n, err := NewDirectory()
	if err != nil {
		return nil, fmt.Errorf("node: create dir: %w", err)
	}
	if err := s.repo.Save(ctx, n); err != nil {
		return nil, fmt.Errorf("node: save dir: %w", err)
	}
	return n, nil
}

// CreateSymlink creates and persists a symlink node.
func (s *Service) CreateSymlink(ctx context.Context, target string) (*Node, error) {
	n, err := NewSymlink(target)
	if err != nil {
		return nil, fmt.Errorf("node: create symlink: %w", err)
	}
	if err := s.repo.Save(ctx, n); err != nil {
		return nil, fmt.Errorf("node: save symlink: %w", err)
	}
	return n, nil
}

// CreateObject creates and persists an object (S3-backed) node.
func (s *Service) CreateObject(ctx context.Context, content ObjectContent, size int64) (*Node, error) {
	n, err := NewObject(content, size)
	if err != nil {
		return nil, fmt.Errorf("node: create object: %w", err)
	}
	if err := s.repo.Save(ctx, n); err != nil {
		return nil, fmt.Errorf("node: save object: %w", err)
	}
	return n, nil
}

// Link adds a child entry to parent and persists the parent.
// Increments child's nlink (POSIX hardlink semantics). A fresh child
// (nlink==0 from newNode) gets nlink=1 after its first Link; subsequent
// Links add to the count.
func (s *Service) Link(ctx context.Context, parent *Node, name string, child *Node) error {
	if parent == nil || child == nil {
		return fmt.Errorf("node: link: nil parent or child")
	}
	if err := parent.AddEntry(name, child); err != nil {
		return fmt.Errorf("node: link: %w", err)
	}
	if err := s.repo.Save(ctx, parent); err != nil {
		return err
	}
	child.nlink++
	now := time.Now()
	child.ctime = now
	child.rev = child.rev.Next()
	return s.repo.Save(ctx, child)
}

// Unlink removes a child entry from parent and decrements child's nlink.
// If nlink reaches zero, the child is deleted. Returns the deleted node
// (caller may use it for S3 cleanup) or nil if only the link was removed.
func (s *Service) Unlink(ctx context.Context, parent *Node, name string) (*Node, error) {
	if parent == nil {
		return nil, fmt.Errorf("node: unlink: nil parent")
	}
	// Read entry first to capture child id before removal.
	dc, err := parent.ReadDir()
	if err != nil {
		return nil, fmt.Errorf("node: unlink: read dir: %w", err)
	}
	var childID uuid.UUID
	var found bool
	for _, e := range dc.Entries {
		if e.Name == name {
			childID = e.InodeID
			found = true
			break
		}
	}
	if !found {
		return nil, ErrEntryNotFound
	}
	if err := parent.RemoveEntry(name); err != nil {
		return nil, fmt.Errorf("node: unlink: %w", err)
	}
	if err := s.repo.Save(ctx, parent); err != nil {
		return nil, err
	}
	if childID == uuid.Nil {
		return nil, nil
	}
	child, err := s.GetByID(ctx, childID)
	if err != nil {
		return nil, fmt.Errorf("node: unlink: get child: %w", err)
	}
	if child.nlink > 1 {
		child.nlink--
		now := time.Now()
		child.ctime = now
		child.rev = child.rev.Next()
		if err := s.repo.Save(ctx, child); err != nil {
			return nil, err
		}
		return nil, nil
	}
	// nlink==1 -> last reference; delete the child.
	if err := s.repo.Delete(ctx, childID); err != nil {
		return nil, fmt.Errorf("node: unlink: delete: %w", err)
	}
	return child, nil
}

// UnlinkOrReplace removes a child entry from parent if one exists at name,
// and, if a child node was present, decrements its nlink (deleting at 0).
// If no entry exists, returns (nil, nil) — the operation is a no-op, suitable
// for overwrite semantics where absence is the success case.
// If the existing target is a directory, returns ErrIsDirectory (POSIX:
// cannot overwrite a directory with a non-directory).
//
// Returns the deleted child (caller may use it for S3 cleanup) or nil.
func (s *Service) UnlinkOrReplace(ctx context.Context, parent *Node, name string) (*Node, error) {
	if parent == nil {
		return nil, fmt.Errorf("node: unlink: nil parent")
	}
	entry, err := parent.Lookup(name)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, nil
	}
	child, err := s.GetByID(ctx, entry.InodeID)
	if err != nil {
		return nil, fmt.Errorf("unlink: get existing: %w", err)
	}
	if child.IsDir() {
		return nil, ErrIsDirectory
	}
	return s.Unlink(ctx, parent, name)
}

// GetByID returns a node by its ID.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*Node, error) {
	n, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("node: get: %w", err)
	}
	return n, nil
}

// Update saves a node after its content has been mutated.
func (s *Service) Update(ctx context.Context, n *Node) error {
	if n == nil {
		return fmt.Errorf("node: update: nil node")
	}
	return s.repo.Save(ctx, n)
}

// Save persists the node (alias for Update).
func (s *Service) Save(ctx context.Context, n *Node) error {
	return s.repo.Save(ctx, n)
}

// Delete removes a node by its ID.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// WithTx executes fn within a transaction.
func (s *Service) WithTx(ctx context.Context, fn func(*Service) error) error {
	return s.repo.WithTx(ctx, func(txRepo Repository) error {
		return fn(&Service{repo: txRepo})
	})
}
