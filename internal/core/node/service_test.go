package node

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRepo is a minimal in-memory Repository for testing nlink logic.
type fakeRepo struct {
	nodes   map[uuid.UUID]*Node
	deleted map[uuid.UUID]bool
	saveErr error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{nodes: map[uuid.UUID]*Node{}, deleted: map[uuid.UUID]bool{}}
}

func (r *fakeRepo) Get(_ context.Context, id uuid.UUID) (*Node, error) {
	if r.deleted[id] {
		return nil, ErrNotFound
	}
	n, ok := r.nodes[id]
	if !ok {
		return nil, ErrNotFound
	}
	return n, nil
}

func (r *fakeRepo) Save(_ context.Context, n *Node) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	r.nodes[n.id] = n
	return nil
}

func (r *fakeRepo) Delete(_ context.Context, id uuid.UUID) error {
	r.deleted[id] = true
	delete(r.nodes, id)
	return nil
}

func (r *fakeRepo) WithTx(_ context.Context, fn func(Repository) error) error {
	return fn(r)
}

func TestLinkIncrementsNLink(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)

	dir, err := NewDirectory()
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), dir))

	child, err := NewFile("hello")
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), child))
	require.Equal(t, uint32(0), child.NLink(), "freshly created node has nlink=0 (POSIX)")

	require.NoError(t, svc.Link(context.Background(), dir, "a", child))
	assert.Equal(t, uint32(1), child.NLink(), "first Link should set nlink=1")

	// Second link from same parent: must reject (entry already exists), nlink unchanged.
	err = svc.Link(context.Background(), dir, "a", child)
	assert.ErrorIs(t, err, ErrEntryExists, "duplicate link in same dir must fail")
	assert.Equal(t, uint32(1), child.NLink())
}

func TestUnlinkDecrementsAndDeletes(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)

	dir, err := NewDirectory()
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), dir))

	child, err := NewFile("hello")
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), child))

	require.NoError(t, svc.Link(context.Background(), dir, "a", child))
	assert.Equal(t, uint32(1), child.NLink())

	// Add a 2nd hardlink from another parent.
	dir2, err := NewDirectory()
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), dir2))
	require.NoError(t, svc.Link(context.Background(), dir2, "b", child))
	assert.Equal(t, uint32(2), child.NLink())

	// Unlink only decrements (nlink 2 -> 1), child remains.
	deleted, err := svc.Unlink(context.Background(), dir, "a")
	require.NoError(t, err)
	assert.Nil(t, deleted, "nlink>1 should not delete the child")
	assert.Equal(t, uint32(1), child.NLink())
	_, err = repo.Get(context.Background(), child.ID())
	assert.NoError(t, err, "child must still exist after decrement")
}

func TestUnlinkLastLinkDeletes(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)

	dir, err := NewDirectory()
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), dir))

	child, err := NewFile("hello")
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), child))

	// Single Link -> nlink=1.
	require.NoError(t, svc.Link(context.Background(), dir, "a", child))
	assert.Equal(t, uint32(1), child.NLink())

	// Unlink the only entry: nlink 1 -> 0, child must be deleted.
	deleted, err := svc.Unlink(context.Background(), dir, "a")
	require.NoError(t, err)
	assert.NotNil(t, deleted, "nlink=1 -> 0 must return deleted child")
	assert.Equal(t, child.ID(), deleted.ID())
	_, err = repo.Get(context.Background(), child.ID())
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestUnlinkOrReplaceRejectsDirectory(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)

	parent, err := NewDirectory()
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), parent))

	subdir, err := NewDirectory()
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), subdir))

	require.NoError(t, svc.Link(context.Background(), parent, "d", subdir))

	_, err = svc.UnlinkOrReplace(context.Background(), parent, "d")
	assert.ErrorIs(t, err, ErrIsDirectory, "directory target must be rejected")
}

func TestUnlinkOrReplaceNoEntry(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)

	dir, err := NewDirectory()
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), dir))

	deleted, err := svc.UnlinkOrReplace(context.Background(), dir, "absent")
	assert.NoError(t, err)
	assert.Nil(t, deleted)
}

func TestBulkLinkSingleWrite(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)

	dir, err := NewDirectory()
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), dir))

	children := map[string]*Node{}
	for _, name := range []string{"a", "b", "c"} {
		c, err := NewFile(name)
		require.NoError(t, err)
		require.NoError(t, repo.Save(context.Background(), c))
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
	repo := newFakeRepo()
	svc := NewService(repo)

	dir, err := NewDirectory()
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), dir))

	first, err := NewFile("a")
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), first))
	require.NoError(t, svc.Link(context.Background(), dir, "a", first))

	second, err := NewFile("a2")
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), second))

	err = svc.BulkLink(context.Background(), dir, map[string]*Node{"a": second})
	assert.ErrorIs(t, err, ErrEntryExists)
	// first was already linked; second must not have been touched.
	assert.Equal(t, uint32(1), first.NLink())
	assert.Equal(t, uint32(0), second.NLink())
}

func TestBulkUnlinkDropsEntries(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)

	dir, err := NewDirectory()
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), dir))

	children := map[string]*Node{}
	for _, name := range []string{"a", "b", "c"} {
		c, err := NewFile(name)
		require.NoError(t, err)
		require.NoError(t, repo.Save(context.Background(), c))
		children[name] = c
		require.NoError(t, svc.Link(context.Background(), dir, name, c))
	}

	deleted, err := svc.BulkUnlink(context.Background(), dir, []string{"a", "b", "absent"})
	require.NoError(t, err)
	assert.Len(t, deleted, 2, "a and b were the only refs, must be deleted")
	for _, d := range deleted {
		_, err := repo.Get(context.Background(), d.ID())
		assert.ErrorIs(t, err, ErrNotFound)
	}
	// c remains.
	entry, err := dir.Lookup("c")
	require.NoError(t, err)
	require.NotNil(t, entry)
	assert.Equal(t, children["c"].ID(), entry.InodeID)
}

func TestBulkUnlinkPartialRefs(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)

	dir, err := NewDirectory()
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), dir))

	dir2, err := NewDirectory()
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), dir2))

	child, err := NewFile("shared")
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), child))
	require.NoError(t, svc.Link(context.Background(), dir, "x", child))
	require.NoError(t, svc.Link(context.Background(), dir2, "x", child))
	assert.Equal(t, uint32(2), child.NLink())

	// Remove the first hardlink via BulkUnlink: child must remain (nlink=1).
	deleted, err := svc.BulkUnlink(context.Background(), dir, []string{"x"})
	require.NoError(t, err)
	assert.Empty(t, deleted)
	assert.Equal(t, uint32(1), child.NLink())
	_, err = repo.Get(context.Background(), child.ID())
	assert.NoError(t, err)

	// Remove the second (last) hardlink: child deleted.
	deleted, err = svc.BulkUnlink(context.Background(), dir2, []string{"x"})
	require.NoError(t, err)
	assert.Len(t, deleted, 1)
	assert.Equal(t, child.ID(), deleted[0].ID())
	_, err = repo.Get(context.Background(), child.ID())
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestMoveEntryRenameWithinDir(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)

	dir, err := NewDirectory()
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), dir))

	file, err := NewFile("a")
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), file))
	require.NoError(t, svc.Link(context.Background(), dir, "a", file))

	// Rename a → b within the same dir. The inode's nlink must stay
	// at 1 (not bump to 2 like the old Unlink+Link pair would do).
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
	repo := newFakeRepo()
	svc := NewService(repo)

	dir, err := NewDirectory()
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), dir))

	a, err := NewFile("a")
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), a))
	require.NoError(t, svc.Link(context.Background(), dir, "a", a))

	b, err := NewFile("b")
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), b))
	require.NoError(t, svc.Link(context.Background(), dir, "b", b))

	// Mv /a → /b (overwrite). /a is gone, /b's inode is gone, /a's
	// inode is now at "b" with nlink=1.
	require.NoError(t, svc.MoveEntry(context.Background(), dir, "a", dir, "b"))

	entry, err := dir.Lookup("a")
	require.NoError(t, err)
	assert.Nil(t, entry)

	entry, err = dir.Lookup("b")
	require.NoError(t, err)
	require.NotNil(t, entry)
	assert.Equal(t, a.ID(), entry.InodeID, "b's entry should now point at a's inode")
	assert.Equal(t, uint32(1), a.NLink(), "a's inode nlink unchanged")

	// b's inode should be deleted (nlink was 1, the move decremented by overwrite).
	_, err = repo.Get(context.Background(), b.ID())
	assert.ErrorIs(t, err, ErrNotFound, "overwritten inode should be deleted")
}

func TestMoveEntryCrossDir(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)

	src, err := NewDirectory()
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), src))

	dst, err := NewDirectory()
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), dst))

	a, err := NewFile("a")
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), a))
	require.NoError(t, svc.Link(context.Background(), src, "a", a))

	// Mv /a → /dst/x (cross-dir move into a directory).
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
	repo := newFakeRepo()
	svc := NewService(repo)

	dir, err := NewDirectory()
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), dir))

	subdir, err := NewDirectory()
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), subdir))
	require.NoError(t, svc.Link(context.Background(), dir, "sub", subdir))

	file, err := NewFile("file")
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), file))
	require.NoError(t, svc.Link(context.Background(), dir, "file", file))

	// Cannot overwrite a directory with a file.
	err = svc.MoveEntry(context.Background(), dir, "file", dir, "sub")
	assert.ErrorIs(t, err, ErrInvalidMoveOverwrite)
}

func TestMoveEntryMissingSource(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)

	dir, err := NewDirectory()
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), dir))

	err = svc.MoveEntry(context.Background(), dir, "absent", dir, "elsewhere")
	assert.ErrorIs(t, err, ErrEntryNotFound)
}
