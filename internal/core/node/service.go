package node

import (
	"context"
	"errors"
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
	return s.create(ctx, "file", func() (*Node, error) { return NewFile(content) })
}

// Touch creates and persists an empty file node, mirroring `touch path`.
// It is a thin convenience over CreateFile with content=""; both return
// a node with an empty FileContent and no further invariants.
func (s *Service) Touch(ctx context.Context) (*Node, error) {
	return s.CreateFile(ctx, "")
}

// CreateDirectory creates and persists a directory node.
func (s *Service) CreateDirectory(ctx context.Context) (*Node, error) {
	return s.create(ctx, "directory", NewDirectory)
}

// CreateSymlink creates and persists a symlink node.
func (s *Service) CreateSymlink(ctx context.Context, target string) (*Node, error) {
	return s.create(ctx, "symlink", func() (*Node, error) { return NewSymlink(target) })
}

// CreateObject creates and persists an object (S3-backed) node.
func (s *Service) CreateObject(ctx context.Context, content ObjectContent, size int64) (*Node, error) {
	return s.create(ctx, "object", func() (*Node, error) { return NewObject(content, size) })
}

// CreateMount creates a mount node pointing to sourceDriveID's root and
// persists it.
func (s *Service) CreateMount(ctx context.Context, sourceDriveID string) (*Node, error) {
	return s.create(ctx, "mount", func() (*Node, error) { return NewMount(sourceDriveID) })
}

// create is the shared persistence step for all Create* methods:
// construct a Node with the type-specific factory, then Save it.
// Centralizing the Save + error wrapping removes the five-line
// "NewX, fmt.Errorf, repo.Save, fmt.Errorf" pattern that would
// otherwise be repeated for each node type.
func (s *Service) create(ctx context.Context, kind string, factory func() (*Node, error)) (*Node, error) {
	n, err := factory()
	if err != nil {
		return nil, fmt.Errorf("node: create %s: %w", kind, err)
	}
	if err := s.repo.Save(ctx, n); err != nil {
		return nil, fmt.Errorf("node: save %s: %w", kind, err)
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

// BulkLink adds multiple child entries to a single parent in one
// directory write plus one nlink bump per child. The parent is saved
// once; each child is then saved (with nlink++ and revision bump).
// Fails atomically on any conflict (duplicate name, empty name, nil
// child): the directory is left unchanged.
func (s *Service) BulkLink(ctx context.Context, parent *Node, entries map[string]*Node) error {
	if parent == nil {
		return fmt.Errorf("node: bulk link: nil parent")
	}
	if len(entries) == 0 {
		return nil
	}
	if err := parent.AddEntries(entries); err != nil {
		return fmt.Errorf("node: bulk link: %w", err)
	}
	if err := s.repo.Save(ctx, parent); err != nil {
		return err
	}
	now := time.Now()
	for _, child := range entries {
		if child == nil {
			continue
		}
		child.nlink++
		child.ctime = now
		child.rev = child.rev.Next()
		if err := s.repo.Save(ctx, child); err != nil {
			return fmt.Errorf("node: bulk link: save child: %w", err)
		}
	}
	return nil
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

// MoveEntry atomically moves a directory entry from srcParent/srcName
// to dstParent/dstName. The child inode is preserved with its nlink
// unchanged: a move renames the entry, it does not add a new link.
// This is the POSIX rename semantics and avoids the nlink==1
// "delete then re-link" hazard of the Unlink+Link pair, which would
// otherwise drop and recreate the inode.
//
// If dstParent/dstName already points to a different inode, that
// inode is overwritten: its directory entry is removed and its
// nlink is decremented (or the inode is deleted if nlink hits 0).
// Type mismatch between src and overwrite-target is rejected
// (POSIX: cannot overwrite a directory with a non-directory).
//
// Returns ErrEntryNotFound if srcName is not in srcParent.
func (s *Service) MoveEntry(ctx context.Context, srcParent *Node, srcName string, dstParent *Node, dstName string) error {
	if srcParent == nil || dstParent == nil {
		return fmt.Errorf("node: move entry: nil parent")
	}

	srcDC, err := srcParent.ReadDir()
	if err != nil {
		return fmt.Errorf("move entry: read src dir: %w", err)
	}
	var (
		srcInodeID uuid.UUID
		srcType    NodeType
		srcFound   bool
	)
	for _, e := range srcDC.Entries {
		if e.Name == srcName {
			srcInodeID = e.InodeID
			srcType = e.Type
			srcFound = true
			break
		}
	}
	if !srcFound {
		return ErrEntryNotFound
	}

	// No-op rename: same parent and same name. Returning early
	// avoids touching the DB and keeps callers from having to special-case
	// the identity operation.
	if srcParent.ID() == dstParent.ID() && srcName == dstName {
		return nil
	}

	// Find any existing entry at dstName so we can decide whether
	// this is an overwrite and (if so) of what type.
	var (
		existingInodeID    uuid.UUID
		existingType       NodeType
		existingInodeKnown bool
	)
	dstDC, err := dstParent.ReadDir()
	if err != nil {
		return fmt.Errorf("move entry: read dst dir: %w", err)
	}
	for _, e := range dstDC.Entries {
		if e.Name == dstName {
			existingInodeID = e.InodeID
			existingType = e.Type
			existingInodeKnown = true
			break
		}
	}

	// If the existing dst entry is the same inode as src, this is a
	// no-op (idempotent rename of a path that already points to the
	// right inode). Save both parents in case mtime/ctime matter.
	if existingInodeKnown && existingInodeID == srcInodeID {
		return nil
	}

	// Type check: cannot overwrite a directory with a non-directory
	// (or vice versa). Same-type overwrites are fine.
	if existingInodeKnown && existingType != srcType {
		return ErrInvalidMoveOverwrite
	}

	// Build the updated directory listing(s). When src and dst share
	// a parent (rename within a directory), we update that one
	// directory once; otherwise we update each independently.
	// Doing it as a single update when srcParent == dstParent is
	// important: the two-pass approach (write dst, then write src)
	// would overwrite the dst update with the unmodified src content.
	newSrcEntries := make([]DirEntry, 0, len(srcDC.Entries)+1)
	dstEntryReplaced := false
	for _, e := range srcDC.Entries {
		if e.Name == srcName {
			continue
		}
		if srcParent.ID() == dstParent.ID() && e.Name == dstName {
			// Same parent: replace this entry with the renamed one.
			newSrcEntries = append(newSrcEntries, DirEntry{
				InodeID: srcInodeID,
				Name:    dstName,
				Type:    srcType,
			})
			dstEntryReplaced = true
			continue
		}
		newSrcEntries = append(newSrcEntries, e)
	}
	if srcParent.ID() == dstParent.ID() && !dstEntryReplaced {
		newSrcEntries = append(newSrcEntries, DirEntry{
			InodeID: srcInodeID,
			Name:    dstName,
			Type:    srcType,
		})
	}
	if srcParent.ID() == dstParent.ID() {
		if err := srcParent.WriteDir(DirContent{Entries: newSrcEntries}); err != nil {
			return fmt.Errorf("move entry: write parent: %w", err)
		}
		if err := s.repo.Save(ctx, srcParent); err != nil {
			return fmt.Errorf("move entry: save parent: %w", err)
		}
	} else {
		newDstEntries := make([]DirEntry, 0, len(dstDC.Entries)+1)
		for _, e := range dstDC.Entries {
			if e.Name == dstName {
				continue
			}
			newDstEntries = append(newDstEntries, e)
		}
		newDstEntries = append(newDstEntries, DirEntry{
			InodeID: srcInodeID,
			Name:    dstName,
			Type:    srcType,
		})
		if err := srcParent.WriteDir(DirContent{Entries: newSrcEntries}); err != nil {
			return fmt.Errorf("move entry: write src dir: %w", err)
		}
		if err := s.repo.Save(ctx, srcParent); err != nil {
			return fmt.Errorf("move entry: save src: %w", err)
		}
		if err := dstParent.WriteDir(DirContent{Entries: newDstEntries}); err != nil {
			return fmt.Errorf("move entry: write dst dir: %w", err)
		}
		if err := s.repo.Save(ctx, dstParent); err != nil {
			return fmt.Errorf("move entry: save dst: %w", err)
		}
	}

	// Handle the overwrite target: drop its directory entry (done
	// above) and decrement its nlink; delete the inode if its
	// nlink reaches 0. The src inode's nlink is untouched because
	// the move just relocates an existing link, it does not create
	// a new one.
	if existingInodeKnown {
		overwrite, err := s.GetByID(ctx, existingInodeID)
		if err != nil {
			return fmt.Errorf("move entry: load overwrite target: %w", err)
		}
		if overwrite.nlink <= 1 {
			if err := s.repo.Delete(ctx, existingInodeID); err != nil {
				return fmt.Errorf("move entry: delete overwritten inode: %w", err)
			}
		} else {
			overwrite.nlink--
			overwrite.rev = overwrite.rev.Next()
			if err := s.repo.Save(ctx, overwrite); err != nil {
				return fmt.Errorf("move entry: save overwritten inode: %w", err)
			}
		}
	}
	return nil
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

// BulkUnlink removes multiple entries from a single parent in one
// directory write. For each removed child, nlink is decremented and
// the child is deleted if nlink reaches zero. Returns the deleted
// children (so callers can enqueue S3 tombstones).
//
// Missing entries are silently ignored (POSIX rm -f semantics).
func (s *Service) BulkUnlink(ctx context.Context, parent *Node, names []string) ([]*Node, error) {
	if parent == nil {
		return nil, fmt.Errorf("node: bulk unlink: nil parent")
	}
	if len(names) == 0 {
		return nil, nil
	}
	// Collect child IDs to decrement *before* mutating the directory.
	dc, err := parent.ReadDir()
	if err != nil {
		return nil, fmt.Errorf("node: bulk unlink: read dir: %w", err)
	}
	byName := make(map[string]uuid.UUID, len(dc.Entries))
	for _, e := range dc.Entries {
		byName[e.Name] = e.InodeID
	}
	seen := make(map[uuid.UUID]bool, len(names))
	type childPlan struct {
		id uuid.UUID
	}
	var planned []childPlan
	for _, n := range names {
		id, ok := byName[n]
		if !ok {
			continue
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		planned = append(planned, childPlan{id: id})
	}
	if err := parent.RemoveEntries(names); err != nil {
		return nil, fmt.Errorf("node: bulk unlink: %w", err)
	}
	if err := s.repo.Save(ctx, parent); err != nil {
		return nil, err
	}
	deleted := make([]*Node, 0, len(planned))
	now := time.Now()
	for _, p := range planned {
		child, err := s.GetByID(ctx, p.id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				continue
			}
			return nil, fmt.Errorf("node: bulk unlink: get child: %w", err)
		}
		if child.nlink > 1 {
			child.nlink--
			child.ctime = now
			child.rev = child.rev.Next()
			if err := s.repo.Save(ctx, child); err != nil {
				return nil, err
			}
			continue
		}
		// nlink==1 -> last reference; delete the child.
		if err := s.repo.Delete(ctx, p.id); err != nil {
			return nil, fmt.Errorf("node: bulk unlink: delete: %w", err)
		}
		deleted = append(deleted, child)
	}
	return deleted, nil
}

// WithTx executes fn within a transaction.
func (s *Service) WithTx(ctx context.Context, fn func(*Service) error) error {
	return s.repo.WithTx(ctx, func(txRepo Repository) error {
		return fn(&Service{repo: txRepo})
	})
}
