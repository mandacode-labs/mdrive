package node

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// Service provides domain-level node operations.
// It wraps Repository with validation and convenience, analogous to
// Linux inode_operations (create, link, unlink, lookup) but without
// path resolution or permission checks — those belong to vfs.
//
// Each multi-step method (Link, Unlink, MoveEntry, BulkLink,
// BulkUnlink) wraps its writes in WithTx so the underlying repository
// state is updated atomically. The in-memory *Node pointers passed
// in may be left in a partially-mutated state if the underlying
// transaction fails; callers should treat a non-nil error as a
// signal to re-fetch any node pointer they intend to use again.
type Service struct {
	repo Repository
}

// NewService creates a Service.
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
//
// Callers that need atomic create+link (i.e. so a partial failure
// cannot leave an orphan node) must construct the node via
// newNode() directly and pass it to Link, which inserts the
// child inside the same transaction as the parent update.
func (s *Service) create(ctx context.Context, kind string, factory func() (*Node, error)) (*Node, error) {
	n, err := factory()
	if err != nil {
		return nil, errorx.New(errorx.KindBadRequest, fmt.Sprintf("node: create %s factory failed", kind))
	}
	if err := s.repo.Save(ctx, n); err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("node: save %s", kind))
	}
	return n, nil
}

// Link adds a child entry to parent and persists the parent.
// Increments child's nlink (POSIX hardlink semantics). A fresh child
// (nlink==0 from newNode) gets nlink=1 after its first Link; subsequent
// Links add to the count.
//
// On success, child is mutated in place to reflect the new state
// (nlink, ctime, rev). On failure, child's state is undefined — callers
// should discard the pointer.
func (s *Service) Link(ctx context.Context, parent *Node, name string, child *Node) error {
	if parent == nil || child == nil {
		return errorx.New(errorx.KindBadRequest, "node: link requires non-nil parent and child")
	}
	return s.WithTx(ctx, func(tx *Service) error {
		if err := parent.AddEntry(name, child); err != nil {
			return errorx.Wrap(err, fmt.Sprintf("node: link add entry (name=%s)", name))
		}
		if err := tx.repo.Save(ctx, parent); err != nil {
			return errorx.Wrap(err, "node: link save parent")
		}
		child.nlink++
		now := time.Now()
		child.ctime = now
		child.rev = child.rev.Next()
		return errorx.Wrap(tx.repo.Save(ctx, child), "node: link save child")
	})
}

// BulkLink adds multiple child entries to a single parent in one
// directory write plus one nlink bump per child. The parent is saved
// once; each child is then saved (with nlink++ and revision bump).
// Fails atomically on any conflict (duplicate name, empty name, nil
// child): the directory is left unchanged.
func (s *Service) BulkLink(ctx context.Context, parent *Node, entries map[string]*Node) error {
	if parent == nil {
		return errorx.New(errorx.KindBadRequest, "node: bulk link requires non-nil parent")
	}
	if len(entries) == 0 {
		return nil
	}
	return s.WithTx(ctx, func(tx *Service) error {
		if err := parent.AddEntries(entries); err != nil {
			return errorx.Wrap(err, fmt.Sprintf("node: bulk link add entries (count=%d)", len(entries)))
		}
		if err := tx.repo.Save(ctx, parent); err != nil {
			return errorx.Wrap(err, "node: bulk link save parent")
		}
		now := time.Now()
		for name, child := range entries {
			if child == nil {
				continue
			}
			child.nlink++
			child.ctime = now
			child.rev = child.rev.Next()
			if err := tx.repo.Save(ctx, child); err != nil {
				return errorx.Wrap(err, fmt.Sprintf("node: bulk link save child (name=%s)", name))
			}
		}
		return nil
	})
}

// Unlink removes a child entry from parent and decrements child's nlink.
// If nlink reaches zero, the child is deleted. Returns the deleted node
// (caller may use it for S3 cleanup) or nil if only the link was removed.
func (s *Service) Unlink(ctx context.Context, parent *Node, name string) (*Node, error) {
	if parent == nil {
		return nil, errorx.New(errorx.KindBadRequest, "node: unlink requires non-nil parent")
	}
	var deleted *Node
	err := s.WithTx(ctx, func(tx *Service) error {
		dc, err := parent.ReadDir()
		if err != nil {
			return errorx.Wrap(err, fmt.Sprintf("node: unlink read dir (name=%s)", name))
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
			return errorx.New(errorx.KindNotFound, "node: entry not found")
		}
		if err := parent.RemoveEntry(name); err != nil {
			return errorx.Wrap(err, fmt.Sprintf("node: unlink remove entry (name=%s)", name))
		}
		if err := tx.repo.Save(ctx, parent); err != nil {
			return errorx.Wrap(err, fmt.Sprintf("node: unlink save parent (name=%s)", name))
		}
		if childID == uuid.Nil {
			return nil
		}
		child, err := tx.GetByID(ctx, childID)
		if err != nil {
			return errorx.Wrap(err, fmt.Sprintf("node: unlink get child (name=%s, child_id=%s)", name, childID))
		}
		if child.nlink > 1 {
			child.nlink--
			now := time.Now()
			child.ctime = now
			child.rev = child.rev.Next()
			return errorx.Wrap(tx.repo.Save(ctx, child), "node: unlink decrement child nlink")
		}
		if err := tx.repo.Delete(ctx, childID); err != nil {
			return errorx.Wrap(err, fmt.Sprintf("node: unlink delete child (child_id=%s)", childID))
		}
		deleted = child
		return nil
	})
	if err != nil {
		return nil, err
	}
	return deleted, nil
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
		return nil, errorx.New(errorx.KindBadRequest, "node: unlink requires non-nil parent")
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
		return nil, errorx.Wrap(err, fmt.Sprintf("node: unlink_or_replace get existing (name=%s, inode_id=%s)", name, entry.InodeID))
	}
	if child.IsDir() {
		return nil, errorx.New(errorx.KindBadRequest, "node: target is a directory")
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
		return errorx.New(errorx.KindBadRequest, "node: move entry requires non-nil parents")
	}
	return s.WithTx(ctx, func(tx *Service) error {
		srcDC, err := srcParent.ReadDir()
		if err != nil {
			return errorx.Wrap(err, fmt.Sprintf("move entry read src dir (src_name=%s)", srcName))
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
			return errorx.New(errorx.KindNotFound, "node: entry not found")
		}

		if srcParent.ID() == dstParent.ID() && srcName == dstName {
			return nil
		}

		var (
			existingInodeID    uuid.UUID
			existingType       NodeType
			existingInodeKnown bool
		)
		dstDC, err := dstParent.ReadDir()
		if err != nil {
			return errorx.Wrap(err, fmt.Sprintf("move entry read dst dir (dst_name=%s)", dstName))
		}
		for _, e := range dstDC.Entries {
			if e.Name == dstName {
				existingInodeID = e.InodeID
				existingType = e.Type
				existingInodeKnown = true
				break
			}
		}

		if existingInodeKnown && existingInodeID == srcInodeID {
			return nil
		}

		if existingInodeKnown && existingType != srcType {
			return errorx.New(errorx.KindBadRequest, "node: cannot overwrite entry of different type")
		}

		newSrcEntries := make([]DirEntry, 0, len(srcDC.Entries)+1)
		dstEntryReplaced := false
		for _, e := range srcDC.Entries {
			if e.Name == srcName {
				continue
			}
			if srcParent.ID() == dstParent.ID() && e.Name == dstName {
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
				return errorx.Wrap(err, "move entry write parent")
			}
			if err := tx.repo.Save(ctx, srcParent); err != nil {
				return errorx.Wrap(err, "move entry save parent")
			}
		} else {
			newDstEntries := make([]DirEntry, 0, len(dstDC.Entries)+1)
			for _, e := range dstDC.Entries {
				if e.Name == dstName {
					continue
				}
			}
			newDstEntries = append(newDstEntries, DirEntry{
				InodeID: srcInodeID,
				Name:    dstName,
				Type:    srcType,
			})
			if err := srcParent.WriteDir(DirContent{Entries: newSrcEntries}); err != nil {
				return errorx.Wrap(err, "move entry write src dir")
			}
			if err := tx.repo.Save(ctx, srcParent); err != nil {
				return errorx.Wrap(err, "move entry save src")
			}
			if err := dstParent.WriteDir(DirContent{Entries: newDstEntries}); err != nil {
				return errorx.Wrap(err, "move entry write dst dir")
			}
			if err := tx.repo.Save(ctx, dstParent); err != nil {
				return errorx.Wrap(err, "move entry save dst")
			}
		}

		if existingInodeKnown {
			overwrite, err := tx.GetByID(ctx, existingInodeID)
			if err != nil {
				return errorx.Wrap(err, fmt.Sprintf("move entry load overwrite target (inode_id=%s)", existingInodeID))
			}
			if overwrite.nlink <= 1 {
				if err := tx.repo.Delete(ctx, existingInodeID); err != nil {
					return errorx.Wrap(err, fmt.Sprintf("move entry delete overwritten inode (inode_id=%s)", existingInodeID))
				}
			} else {
				overwrite.nlink--
				overwrite.rev = overwrite.rev.Next()
				if err := tx.repo.Save(ctx, overwrite); err != nil {
					return errorx.Wrap(err, "move entry save overwritten inode")
				}
			}
		}
		return nil
	})
}

// GetByID returns a node by its ID.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*Node, error) {
	n, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("node: get (id=%s)", id))
	}
	return n, nil
}

// Save persists a node after its content has been mutated. The
// repository's Save handles both insert (for a fresh inode) and
// update (for an existing one) based on the node's staleRev.
func (s *Service) Save(ctx context.Context, n *Node) error {
	if n == nil {
		return errorx.New(errorx.KindBadRequest, "node: save requires non-nil node")
	}
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
		return nil, errorx.New(errorx.KindBadRequest, "node: bulk unlink requires non-nil parent")
	}
	if len(names) == 0 {
		return nil, nil
	}
	var deleted []*Node
	err := s.WithTx(ctx, func(tx *Service) error {
		dc, err := parent.ReadDir()
		if err != nil {
			return errorx.Wrap(err, "node: bulk unlink read dir")
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
			return errorx.Wrap(err, fmt.Sprintf("node: bulk unlink remove entries (count=%d)", len(names)))
		}
		if err := tx.repo.Save(ctx, parent); err != nil {
			return errorx.Wrap(err, "node: bulk unlink save parent")
		}
		now := time.Now()
		for _, p := range planned {
			child, err := tx.GetByID(ctx, p.id)
			if err != nil {
				if errorx.KindOf(err) == errorx.KindNotFound {
					continue
				}
				return errorx.Wrap(err, fmt.Sprintf("node: bulk unlink get child (id=%s)", p.id))
			}
			if child.nlink > 1 {
				child.nlink--
				child.ctime = now
				child.rev = child.rev.Next()
				if err := tx.repo.Save(ctx, child); err != nil {
					return errorx.Wrap(err, "node: bulk unlink save child nlink decrement")
				}
				continue
			}
			if err := tx.repo.Delete(ctx, p.id); err != nil {
				return errorx.Wrap(err, fmt.Sprintf("node: bulk unlink delete (id=%s)", p.id))
			}
			deleted = append(deleted, child)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return deleted, nil
}

// WithTx executes fn within a transaction. The transaction spans only
// the node repository; callers needing cross-repository atomicity
// must coordinate at a higher layer.
func (s *Service) WithTx(ctx context.Context, fn func(*Service) error) error {
	return s.repo.WithTx(ctx, func(txRepo Repository) error {
		return fn(&Service{repo: txRepo})
	})
}
