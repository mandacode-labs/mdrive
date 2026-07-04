package node

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/entx"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/logx"
)

// NodeOperation is the public contract of the node domain. In Linux
// terms it is the equivalent of inode_operations / file_operations:
// the per-inode actions (create, link, unlink, move, save, delete,
// lookup) composed into a single interface. Path resolution and
// permission checks live in vfs.
//
// Multi-step methods (Link, Unlink, MoveEntry, BulkLink, BulkUnlink)
// wrap their writes in WithTx; on tx failure, *Node pointers passed
// in may be partially mutated and must be re-fetched.
type NodeOperation interface {
	CreateFile(ctx context.Context, content string) (*Node, error)
	Touch(ctx context.Context) (*Node, error)
	CreateDirectory(ctx context.Context) (*Node, error)
	CreateSymlink(ctx context.Context, target string) (*Node, error)
	CreateObject(ctx context.Context, content ObjectContent, size int64) (*Node, error)
	CreateMount(ctx context.Context, sourceDriveID string) (*Node, error)
	Link(ctx context.Context, parent *Node, name string, child *Node) error
	BulkLink(ctx context.Context, parent *Node, entries map[string]*Node) error
	Unlink(ctx context.Context, parent *Node, name string) (*Node, error)
	UnlinkOrReplace(ctx context.Context, parent *Node, name string) (*Node, error)
	MoveEntry(ctx context.Context, srcParent *Node, srcName string, dstParent *Node, dstName string) error
	GetByID(ctx context.Context, id uuid.UUID) (*Node, error)
	Save(ctx context.Context, n *Node) error
	Delete(ctx context.Context, id uuid.UUID) error
	BulkUnlink(ctx context.Context, parent *Node, names []string) ([]*Node, error)
}

// nodeOperation is the only implementation of NodeOperation.
type nodeOperation struct {
	repo Repository
	tm   entx.TxManager
}

// NewNodeOperation wires the node domain.
func NewNodeOperation(repo Repository, tm entx.TxManager) NodeOperation {
	return &nodeOperation{repo: repo, tm: tm}
}

var _ NodeOperation = (*nodeOperation)(nil)

func (s *nodeOperation) CreateFile(ctx context.Context, content string) (*Node, error) {
	return s.create(ctx, "file", func() (*Node, error) { return NewFile(content) })
}

func (s *nodeOperation) Touch(ctx context.Context) (*Node, error) {
	return s.CreateFile(ctx, "")
}

func (s *nodeOperation) CreateDirectory(ctx context.Context) (*Node, error) {
	return s.create(ctx, "directory", NewDirectory)
}

func (s *nodeOperation) CreateSymlink(ctx context.Context, target string) (*Node, error) {
	return s.create(ctx, "symlink", func() (*Node, error) { return NewSymlink(target) })
}

func (s *nodeOperation) CreateObject(ctx context.Context, content ObjectContent, size int64) (*Node, error) {
	return s.create(ctx, "object", func() (*Node, error) { return NewObject(content, size) })
}

func (s *nodeOperation) CreateMount(ctx context.Context, sourceDriveID string) (*Node, error) {
	return s.create(ctx, "mount", func() (*Node, error) { return NewMount(sourceDriveID) })
}

func (s *nodeOperation) create(ctx context.Context, kind string, factory func() (*Node, error)) (*Node, error) {
	logx.Debug(ctx, "node.op.create.enter", slog.String("kind", kind))
	n, err := factory()
	if err != nil {
		return nil, errorx.New(errorx.KindInternal, fmt.Sprintf("node: create %s factory failed", kind))
	}
	if err := s.repo.Save(ctx, n); err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("node: save %s", kind))
	}
	logx.Debug(ctx, "node.op.create.ok",
		slog.String("kind", kind),
		slog.String("id", n.ID().String()),
	)
	return n, nil
}

func (s *nodeOperation) Link(ctx context.Context, parent *Node, name string, child *Node) error {
	logx.Debug(ctx, "node.op.link.enter",
		slog.String("parent_id", uuidOrEmpty(parent)),
		slog.String("name", name),
		slog.String("child_id", uuidOrEmpty(child)),
	)
	if parent == nil || child == nil {
		return errorx.New(errorx.KindInvalidArgument, "node: link requires non-nil parent and child")
	}
	return s.tm.WithTx(ctx, func(ctx context.Context) error {
		if err := parent.AddEntry(name, child); err != nil {
			logx.Debug(ctx, "node.op.link.add_entry_err",
				slog.String("name", name),
				slog.String("err", err.Error()),
			)
			return errorx.Wrap(err, fmt.Sprintf("node: link add entry (name=%s)", name))
		}
		if err := s.repo.Save(ctx, parent); err != nil {
			logx.Debug(ctx, "node.op.link.save_parent_err",
				slog.String("name", name),
				slog.String("err", err.Error()),
			)
			return errorx.Wrap(err, "node: link save parent")
		}
		child.nlink++
		now := time.Now()
		child.ctime = now
		child.rev = child.rev.Next()
		if err := s.repo.Save(ctx, child); err != nil {
			return errorx.Wrap(err, "node: link save child")
		}
		logx.Debug(ctx, "node.op.link.ok",
			slog.String("name", name),
			slog.Uint64("nlink", uint64(child.nlink)),
		)
		return nil
	})
}

func (s *nodeOperation) BulkLink(ctx context.Context, parent *Node, entries map[string]*Node) error {
	logx.Debug(ctx, "node.op.bulk_link.enter",
		slog.String("parent_id", uuidOrEmpty(parent)),
		slog.Int("entry_count", len(entries)),
	)
	if parent == nil {
		return errorx.New(errorx.KindInvalidArgument, "node: bulk link requires non-nil parent")
	}
	if len(entries) == 0 {
		return nil
	}
	return s.tm.WithTx(ctx, func(ctx context.Context) error {
		if err := parent.AddEntries(entries); err != nil {
			return errorx.Wrap(err, fmt.Sprintf("node: bulk link add entries (count=%d)", len(entries)))
		}
		if err := s.repo.Save(ctx, parent); err != nil {
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
			if err := s.repo.Save(ctx, child); err != nil {
				return errorx.Wrap(err, fmt.Sprintf("node: bulk link save child (name=%s)", name))
			}
		}
		logx.Debug(ctx, "node.op.bulk_link.ok", slog.Int("entry_count", len(entries)))
		return nil
	})
}

func (s *nodeOperation) Unlink(ctx context.Context, parent *Node, name string) (*Node, error) {
	logx.Debug(ctx, "node.op.unlink.enter",
		slog.String("parent_id", uuidOrEmpty(parent)),
		slog.String("name", name),
	)
	if parent == nil {
		return nil, errorx.New(errorx.KindInvalidArgument, "node: unlink requires non-nil parent")
	}
	var deleted *Node
	err := s.tm.WithTx(ctx, func(ctx context.Context) error {
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
		if err := s.repo.Save(ctx, parent); err != nil {
			return errorx.Wrap(err, fmt.Sprintf("node: unlink save parent (name=%s)", name))
		}
		if childID == uuid.Nil {
			return nil
		}
		child, err := s.GetByID(ctx, childID)
		if err != nil {
			return errorx.Wrap(err, fmt.Sprintf("node: unlink get child (name=%s, child_id=%s)", name, childID))
		}
		if child.nlink > 1 {
			child.nlink--
			now := time.Now()
			child.ctime = now
			child.rev = child.rev.Next()
			return errorx.Wrap(s.repo.Save(ctx, child), "node: unlink decrement child nlink")
		}
		if err := s.repo.Delete(ctx, childID); err != nil {
			return errorx.Wrap(err, fmt.Sprintf("node: unlink delete child (child_id=%s)", childID))
		}
		deleted = child
		return nil
	})
	if err != nil {
		return nil, err
	}
	if deleted != nil {
		logx.Debug(ctx, "node.op.unlink.ok_deleted",
			slog.String("name", name),
			slog.String("deleted_id", deleted.ID().String()),
		)
	} else {
		logx.Debug(ctx, "node.op.unlink.ok_link_removed", slog.String("name", name))
	}
	return deleted, nil
}

func (s *nodeOperation) UnlinkOrReplace(ctx context.Context, parent *Node, name string) (*Node, error) {
	if parent == nil {
		return nil, errorx.New(errorx.KindInvalidArgument, "node: unlink requires non-nil parent")
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
		return nil, errorx.New(errorx.KindFailedPrecondition, "node: target is a directory")
	}
	return s.Unlink(ctx, parent, name)
}

func (s *nodeOperation) MoveEntry(ctx context.Context, srcParent *Node, srcName string, dstParent *Node, dstName string) error {
	logx.Debug(ctx, "node.op.move_entry.enter",
		slog.String("src_parent_id", uuidOrEmpty(srcParent)),
		slog.String("src_name", srcName),
		slog.String("dst_parent_id", uuidOrEmpty(dstParent)),
		slog.String("dst_name", dstName),
	)
	if srcParent == nil || dstParent == nil {
		return errorx.New(errorx.KindInvalidArgument, "node: move entry requires non-nil parents")
	}
	err := s.tm.WithTx(ctx, func(ctx context.Context) error {
		srcDC, err := srcParent.ReadDir()
		if err != nil {
			return errorx.Wrap(err, fmt.Sprintf("move entry read src dir (src_name=%s)", srcName))
		}
		var (
			srcInodeID uuid.UUID
			srcType    NodeKind
			srcFound   bool
		)
		for _, e := range srcDC.Entries {
			if e.Name == srcName {
				srcInodeID = e.InodeID
				srcType = e.Kind
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
			existingType       NodeKind
			existingInodeKnown bool
		)
		dstDC, err := dstParent.ReadDir()
		if err != nil {
			return errorx.Wrap(err, fmt.Sprintf("move entry read dst dir (dst_name=%s)", dstName))
		}
		for _, e := range dstDC.Entries {
			if e.Name == dstName {
				existingInodeID = e.InodeID
				existingType = e.Kind
				existingInodeKnown = true
				break
			}
		}

		if existingInodeKnown && existingInodeID == srcInodeID {
			return nil
		}

		if existingInodeKnown && existingType != srcType {
			return errorx.New(errorx.KindFailedPrecondition, "node: cannot overwrite entry of different type")
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
					Kind:    srcType,
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
				Kind:    srcType,
			})
		}
		if srcParent.ID() == dstParent.ID() {
			if err := srcParent.WriteDir(DirContent{Entries: newSrcEntries}); err != nil {
				return errorx.Wrap(err, "move entry write parent")
			}
			if err := s.repo.Save(ctx, srcParent); err != nil {
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
				Kind:    srcType,
			})
			if err := srcParent.WriteDir(DirContent{Entries: newSrcEntries}); err != nil {
				return errorx.Wrap(err, "move entry write src dir")
			}
			if err := s.repo.Save(ctx, srcParent); err != nil {
				return errorx.Wrap(err, "move entry save src")
			}
			if err := dstParent.WriteDir(DirContent{Entries: newDstEntries}); err != nil {
				return errorx.Wrap(err, "move entry write dst dir")
			}
			if err := s.repo.Save(ctx, dstParent); err != nil {
				return errorx.Wrap(err, "move entry save dst")
			}
		}

		if existingInodeKnown {
			overwrite, err := s.GetByID(ctx, existingInodeID)
			if err != nil {
				return errorx.Wrap(err, fmt.Sprintf("move entry load overwrite target (inode_id=%s)", existingInodeID))
			}
			if overwrite.nlink <= 1 {
				if err := s.repo.Delete(ctx, existingInodeID); err != nil {
					return errorx.Wrap(err, fmt.Sprintf("move entry delete overwritten inode (inode_id=%s)", existingInodeID))
				}
			} else {
				overwrite.nlink--
				overwrite.rev = overwrite.rev.Next()
				if err := s.repo.Save(ctx, overwrite); err != nil {
					return errorx.Wrap(err, "move entry save overwritten inode")
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	logx.Debug(ctx, "node.op.move_entry.ok",
		slog.String("src_name", srcName),
		slog.String("dst_name", dstName),
	)
	return nil
}

func (s *nodeOperation) GetByID(ctx context.Context, id uuid.UUID) (*Node, error) {
	logx.Debug(ctx, "node.op.get.enter", slog.String("id", id.String()))
	n, err := s.repo.Get(ctx, id)
	if err != nil {
		logx.Debug(ctx, "node.op.get.err", slog.String("id", id.String()), slog.String("err", err.Error()))
		return nil, errorx.Wrap(err, fmt.Sprintf("node: get (id=%s)", id))
	}
	logx.Debug(ctx, "node.op.get.ok", slog.String("id", id.String()))
	return n, nil
}

func (s *nodeOperation) Save(ctx context.Context, n *Node) error {
	if n == nil {
		return errorx.New(errorx.KindInvalidArgument, "node: save requires non-nil node")
	}
	logx.Debug(ctx, "node.op.save.enter", slog.String("id", n.ID().String()))
	if err := s.repo.Save(ctx, n); err != nil {
		return errorx.Wrap(err, fmt.Sprintf("node: save (id=%s)", n.ID()))
	}
	return nil
}

func (s *nodeOperation) Delete(ctx context.Context, id uuid.UUID) error {
	logx.Debug(ctx, "node.op.delete.enter", slog.String("id", id.String()))
	if err := s.repo.Delete(ctx, id); err != nil {
		return errorx.Wrap(err, fmt.Sprintf("node: delete (id=%s)", id))
	}
	return nil
}

func (s *nodeOperation) BulkUnlink(ctx context.Context, parent *Node, names []string) ([]*Node, error) {
	logx.Debug(ctx, "node.op.bulk_unlink.enter",
		slog.String("parent_id", uuidOrEmpty(parent)),
		slog.Int("name_count", len(names)),
	)
	if parent == nil {
		return nil, errorx.New(errorx.KindInvalidArgument, "node: bulk unlink requires non-nil parent")
	}
	if len(names) == 0 {
		return nil, nil
	}
	var deleted []*Node
	err := s.tm.WithTx(ctx, func(ctx context.Context) error {
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
		if err := s.repo.Save(ctx, parent); err != nil {
			return errorx.Wrap(err, "node: bulk unlink save parent")
		}
		now := time.Now()
		for _, p := range planned {
			child, err := s.GetByID(ctx, p.id)
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
				if err := s.repo.Save(ctx, child); err != nil {
					return errorx.Wrap(err, "node: bulk unlink save child nlink decrement")
				}
				continue
			}
			if err := s.repo.Delete(ctx, p.id); err != nil {
				return errorx.Wrap(err, fmt.Sprintf("node: bulk unlink delete (id=%s)", p.id))
			}
			deleted = append(deleted, child)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	logx.Debug(ctx, "node.op.bulk_unlink.ok",
		slog.Int("name_count", len(names)),
		slog.Int("deleted_count", len(deleted)),
	)
	return deleted, nil
}

func uuidOrEmpty(n *Node) string {
	if n == nil {
		return ""
	}
	return n.ID().String()
}
