package node_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	nodeMocks "github.com/mandacode-labs/mdrive/internal/core/node/mocks"
	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// memRepo is an in-memory implementation of node.Repository backed by
// a closure-captured map. The mock wrapper delegates to it so tests
// can assert on observable state (nlink, entries, deletions) without
// depending on internal Node fields.
type memRepo struct {
	nodes   map[uuid.UUID]*node.Node
	deleted map[uuid.UUID]bool
}

func newMemRepo() *memRepo {
	return &memRepo{
		nodes:   map[uuid.UUID]*node.Node{},
		deleted: map[uuid.UUID]bool{},
	}
}

func (m *memRepo) get(id uuid.UUID) (*node.Node, bool) {
	if m.deleted[id] {
		return nil, false
	}
	n, ok := m.nodes[id]
	return n, ok
}

func (m *memRepo) save(n *node.Node) {
	m.nodes[n.ID()] = n
}

func (m *memRepo) del(id uuid.UUID) {
	m.deleted[id] = true
	delete(m.nodes, id)
}

// newMockRepo returns a RepositoryMock whose Get/Save/Delete delegate
// to the supplied memRepo. Repository assertions are not enforced; the
// test target is the observable effect on the in-memory state.
func newMockRepo(t *testing.T, mem *memRepo) *nodeMocks.RepositoryMock {
	t.Helper()
	r := nodeMocks.NewRepositoryMock(t)
	r.EXPECT().Get(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, id uuid.UUID) (*node.Node, error) {
			if n, ok := mem.get(id); ok {
				return n, nil
			}
			return nil, errorx.New(errorx.KindNotFound, "node: not found")
		},
	).Maybe()
	r.EXPECT().Save(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, n *node.Node) error {
			mem.save(n)
			return nil
		},
	).Maybe()
	r.EXPECT().Delete(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, id uuid.UUID) error {
			mem.del(id)
			return nil
		},
	).Maybe()
	return r
}

// nopTxManager returns a TxManagerMock that simply invokes fn(ctx).
// Use when the service's transactional wrapping is not the test target.
func nopTxManager(t *testing.T) *nodeMocks.TxManagerMock {
	t.Helper()
	m := nodeMocks.NewTxManagerMock(t)
	m.EXPECT().WithTx(mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, fn func(ctx context.Context) error) error {
			return fn(ctx)
		},
	).Maybe()
	return m
}

func newSvc(t *testing.T) (*node.Service, *memRepo) {
	t.Helper()
	mem := newMemRepo()
	repo := newMockRepo(t, mem)
	svc := node.NewService(repo, nopTxManager(t))
	return svc, mem
}

func TestLinkIncrementsNLink(t *testing.T) {
	svc, mem := newSvc(t)

	dir, err := node.NewDirectory()
	require.NoError(t, err)
	mem.save(dir)

	child, err := node.NewFile("hello")
	require.NoError(t, err)
	mem.save(child)
	require.Equal(t, uint32(0), child.NLink(), "freshly created node has nlink=0 (POSIX)")

	require.NoError(t, svc.Link(context.Background(), dir, "a", child))
	assert.Equal(t, uint32(1), child.NLink(), "first Link should set nlink=1")

	err = svc.Link(context.Background(), dir, "a", child)
	assert.True(t, errorx.IsKind(err, errorx.KindConflict))
	assert.Equal(t, uint32(1), child.NLink())
}

func TestUnlinkDecrementsAndDeletes(t *testing.T) {
	svc, mem := newSvc(t)

	dir, err := node.NewDirectory()
	require.NoError(t, err)
	mem.save(dir)

	child, err := node.NewFile("hello")
	require.NoError(t, err)
	mem.save(child)

	require.NoError(t, svc.Link(context.Background(), dir, "a", child))
	assert.Equal(t, uint32(1), child.NLink())

	dir2, err := node.NewDirectory()
	require.NoError(t, err)
	mem.save(dir2)
	require.NoError(t, svc.Link(context.Background(), dir2, "b", child))
	assert.Equal(t, uint32(2), child.NLink())

	deleted, err := svc.Unlink(context.Background(), dir, "a")
	require.NoError(t, err)
	assert.Nil(t, deleted, "nlink>1 should not delete the child")
	assert.Equal(t, uint32(1), child.NLink())
	_, ok := mem.get(child.ID())
	assert.True(t, ok, "child must still exist after decrement")
}

func TestUnlinkLastLinkDeletes(t *testing.T) {
	svc, mem := newSvc(t)

	dir, err := node.NewDirectory()
	require.NoError(t, err)
	mem.save(dir)

	child, err := node.NewFile("hello")
	require.NoError(t, err)
	mem.save(child)

	require.NoError(t, svc.Link(context.Background(), dir, "a", child))
	assert.Equal(t, uint32(1), child.NLink())

	deleted, err := svc.Unlink(context.Background(), dir, "a")
	require.NoError(t, err)
	assert.NotNil(t, deleted, "nlink=1 -> 0 must return deleted child")
	assert.Equal(t, child.ID(), deleted.ID())
	_, ok := mem.get(child.ID())
	assert.False(t, ok, "deleted child must be gone")
}

func TestUnlinkOrReplaceRejectsDirectory(t *testing.T) {
	svc, mem := newSvc(t)

	parent, err := node.NewDirectory()
	require.NoError(t, err)
	mem.save(parent)

	subdir, err := node.NewDirectory()
	require.NoError(t, err)
	mem.save(subdir)

	require.NoError(t, svc.Link(context.Background(), parent, "d", subdir))

	_, err = svc.UnlinkOrReplace(context.Background(), parent, "d")
	assert.True(t, errorx.IsKind(err, errorx.KindBadRequest))
}

func TestUnlinkOrReplaceNoEntry(t *testing.T) {
	svc, _ := newSvc(t)

	dir, err := node.NewDirectory()
	require.NoError(t, err)

	deleted, err := svc.UnlinkOrReplace(context.Background(), dir, "absent")
	assert.NoError(t, err)
	assert.Nil(t, deleted)
}

func TestBulkLinkSingleWrite(t *testing.T) {
	svc, mem := newSvc(t)

	dir, err := node.NewDirectory()
	require.NoError(t, err)
	mem.save(dir)

	children := map[string]*node.Node{}
	for _, name := range []string{"a", "b", "c"} {
		c, err := node.NewFile(name)
		require.NoError(t, err)
		mem.save(c)
		children[name] = c
	}

	require.NoError(t, svc.BulkLink(context.Background(), dir, children))

	for name, c := range children {
		assert.Equal(t, uint32(1), c.NLink(), "nlink after BulkLink: %s", name)
		entry, err := dir.Lookup(name)
		require.NoError(t, err)
		require.NotNil(t, entry, "entry for %s", name)
		assert.Equal(t, c.ID(), entry.InodeID)
	}
}

func TestBulkLinkRejectsDuplicateName(t *testing.T) {
	svc, mem := newSvc(t)

	dir, err := node.NewDirectory()
	require.NoError(t, err)
	mem.save(dir)

	first, err := node.NewFile("a")
	require.NoError(t, err)
	mem.save(first)
	require.NoError(t, svc.Link(context.Background(), dir, "a", first))

	second, err := node.NewFile("a2")
	require.NoError(t, err)
	mem.save(second)

	err = svc.BulkLink(context.Background(), dir, map[string]*node.Node{"a": second})
	assert.True(t, errorx.IsKind(err, errorx.KindConflict))
	assert.Equal(t, uint32(1), first.NLink())
	assert.Equal(t, uint32(0), second.NLink())
}

func TestBulkUnlinkDropsEntries(t *testing.T) {
	svc, mem := newSvc(t)

	dir, err := node.NewDirectory()
	require.NoError(t, err)
	mem.save(dir)

	children := map[string]*node.Node{}
	for _, name := range []string{"a", "b", "c"} {
		c, err := node.NewFile(name)
		require.NoError(t, err)
		mem.save(c)
		children[name] = c
		require.NoError(t, svc.Link(context.Background(), dir, name, c))
	}

	deleted, err := svc.BulkUnlink(context.Background(), dir, []string{"a", "b", "absent"})
	require.NoError(t, err)
	assert.Len(t, deleted, 2, "a and b were the only refs, must be deleted")
	for _, d := range deleted {
		_, ok := mem.get(d.ID())
		assert.False(t, ok, "deleted child must be gone")
	}
	entry, err := dir.Lookup("c")
	require.NoError(t, err)
	require.NotNil(t, entry)
	assert.Equal(t, children["c"].ID(), entry.InodeID)
}

func TestBulkUnlinkPartialRefs(t *testing.T) {
	svc, mem := newSvc(t)

	dir, err := node.NewDirectory()
	require.NoError(t, err)
	mem.save(dir)

	dir2, err := node.NewDirectory()
	require.NoError(t, err)
	mem.save(dir2)

	child, err := node.NewFile("shared")
	require.NoError(t, err)
	mem.save(child)
	require.NoError(t, svc.Link(context.Background(), dir, "x", child))
	require.NoError(t, svc.Link(context.Background(), dir2, "x", child))
	assert.Equal(t, uint32(2), child.NLink())

	deleted, err := svc.BulkUnlink(context.Background(), dir, []string{"x"})
	require.NoError(t, err)
	assert.Empty(t, deleted)
	assert.Equal(t, uint32(1), child.NLink())
	_, ok := mem.get(child.ID())
	assert.True(t, ok)

	deleted, err = svc.BulkUnlink(context.Background(), dir2, []string{"x"})
	require.NoError(t, err)
	assert.Len(t, deleted, 1)
	assert.Equal(t, child.ID(), deleted[0].ID())
	_, ok = mem.get(child.ID())
	assert.False(t, ok)
}

func TestMoveEntryRenameWithinDir(t *testing.T) {
	svc, mem := newSvc(t)

	dir, err := node.NewDirectory()
	require.NoError(t, err)
	mem.save(dir)

	file, err := node.NewFile("a")
	require.NoError(t, err)
	mem.save(file)
	require.NoError(t, svc.Link(context.Background(), dir, "a", file))

	require.NoError(t, svc.MoveEntry(context.Background(), dir, "a", dir, "b"))

	entry, err := dir.Lookup("a")
	require.NoError(t, err)
	assert.Nil(t, entry, "old name must be gone")

	entry, err = dir.Lookup("b")
	require.NoError(t, err)
	require.NotNil(t, entry, "new name must exist")
	assert.Equal(t, file.ID(), entry.InodeID)
	assert.Equal(t, uint32(1), file.NLink(), "inode nlink must be preserved")
}

func TestMoveEntryOverwriteFile(t *testing.T) {
	svc, mem := newSvc(t)

	dir, err := node.NewDirectory()
	require.NoError(t, err)
	mem.save(dir)

	a, err := node.NewFile("a")
	require.NoError(t, err)
	mem.save(a)
	require.NoError(t, svc.Link(context.Background(), dir, "a", a))

	b, err := node.NewFile("b")
	require.NoError(t, err)
	mem.save(b)
	require.NoError(t, svc.Link(context.Background(), dir, "b", b))

	require.NoError(t, svc.MoveEntry(context.Background(), dir, "a", dir, "b"))

	entry, err := dir.Lookup("a")
	require.NoError(t, err)
	assert.Nil(t, entry)

	entry, err = dir.Lookup("b")
	require.NoError(t, err)
	require.NotNil(t, entry)
	assert.Equal(t, a.ID(), entry.InodeID, "b's entry should now point at a's inode")
	assert.Equal(t, uint32(1), a.NLink(), "a's inode nlink unchanged")

	_, ok := mem.get(b.ID())
	assert.False(t, ok, "b's inode should be deleted")
}

func TestMoveEntryCrossDir(t *testing.T) {
	svc, mem := newSvc(t)

	src, err := node.NewDirectory()
	require.NoError(t, err)
	mem.save(src)

	dst, err := node.NewDirectory()
	require.NoError(t, err)
	mem.save(dst)

	a, err := node.NewFile("a")
	require.NoError(t, err)
	mem.save(a)
	require.NoError(t, svc.Link(context.Background(), src, "a", a))

	require.NoError(t, svc.MoveEntry(context.Background(), src, "a", dst, "x"))

	entry, err := src.Lookup("a")
	require.NoError(t, err)
	assert.Nil(t, entry, "src's a entry should be gone")

	entry, err = dst.Lookup("x")
	require.NoError(t, err)
	require.NotNil(t, entry)
	assert.Equal(t, a.ID(), entry.InodeID)
	assert.Equal(t, uint32(1), a.NLink())
}

func TestMoveEntryRejectsTypeMismatch(t *testing.T) {
	svc, mem := newSvc(t)

	dir, err := node.NewDirectory()
	require.NoError(t, err)
	mem.save(dir)

	subdir, err := node.NewDirectory()
	require.NoError(t, err)
	mem.save(subdir)
	require.NoError(t, svc.Link(context.Background(), dir, "sub", subdir))

	file, err := node.NewFile("file")
	require.NoError(t, err)
	mem.save(file)
	require.NoError(t, svc.Link(context.Background(), dir, "file", file))

	err = svc.MoveEntry(context.Background(), dir, "file", dir, "sub")
	assert.True(t, errorx.IsKind(err, errorx.KindBadRequest))
}

func TestMoveEntryMissingSource(t *testing.T) {
	svc, _ := newSvc(t)

	dir, err := node.NewDirectory()
	require.NoError(t, err)

	err = svc.MoveEntry(context.Background(), dir, "absent", dir, "elsewhere")
	assert.True(t, errorx.IsKind(err, errorx.KindNotFound))
}
