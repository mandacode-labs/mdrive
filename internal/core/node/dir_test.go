package node

import (
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDirectory(t *testing.T) {
	n, err := NewDirectory()
	require.NoError(t, err)

	assert.Equal(t, NodeKindDirectory, n.Kind())

	entries, err := n.ReadDir()
	require.NoError(t, err)
	assert.Len(t, entries.Entries, 0)
}

func TestDirectoryAddRemoveEntry(t *testing.T) {
	dir, err := NewDirectory()
	require.NoError(t, err)

	child, err := NewFile("file content")
	require.NoError(t, err)

	err = dir.AddEntry("foo.txt", child)
	require.NoError(t, err)

	entries, err := dir.ReadDir()
	require.NoError(t, err)
	require.Len(t, entries.Entries, 1)
	assert.Equal(t, "foo.txt", entries.Entries[0].Name)
	assert.Equal(t, child.ID(), entries.Entries[0].InodeID)
	assert.Equal(t, NodeKindFile, entries.Entries[0].Kind)

	err = dir.AddEntry("foo.txt", child)
	assert.True(t, errorx.IsKind(err, errorx.KindConflict))

	e, err := dir.Lookup("foo.txt")
	require.NoError(t, err)
	assert.NotNil(t, e)

	err = dir.RemoveEntry("foo.txt")
	require.NoError(t, err)

	entries, err = dir.ReadDir()
	require.NoError(t, err)
	assert.Len(t, entries.Entries, 0)

	err = dir.RemoveEntry("nope")
	assert.True(t, errorx.IsKind(err, errorx.KindNotFound))
}

func TestAddEntryNotDirectory(t *testing.T) {
	file, err := NewFile("x")
	require.NoError(t, err)

	child, err := NewFile("y")
	require.NoError(t, err)

	err = file.AddEntry("foo", child)
	assert.True(t, errorx.IsKind(err, errorx.KindBadRequest))
}

func TestAddEntryEmptyName(t *testing.T) {
	dir, err := NewDirectory()
	require.NoError(t, err)

	child, err := NewFile("x")
	require.NoError(t, err)

	err = dir.AddEntry("", child)
	assert.True(t, errorx.IsKind(err, errorx.KindBadRequest))
}
