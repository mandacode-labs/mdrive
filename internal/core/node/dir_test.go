package node

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDirectory(t *testing.T) {
	n, err := NewDirectory()
	require.NoError(t, err)

	assert.Equal(t, NodeTypeDirectory, n.Type())

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
	assert.Equal(t, NodeTypeFile, entries.Entries[0].Type)

	err = dir.AddEntry("foo.txt", child)
	assert.ErrorIs(t, err, ErrEntryExists)

	e, err := dir.Lookup("foo.txt")
	require.NoError(t, err)
	assert.NotNil(t, e)

	err = dir.RemoveEntry("foo.txt")
	require.NoError(t, err)

	entries, err = dir.ReadDir()
	require.NoError(t, err)
	assert.Len(t, entries.Entries, 0)

	err = dir.RemoveEntry("nope")
	assert.ErrorIs(t, err, ErrEntryNotFound)
}

func TestAddEntryNotDirectory(t *testing.T) {
	file, err := NewFile("x")
	require.NoError(t, err)

	child, err := NewFile("y")
	require.NoError(t, err)

	err = file.AddEntry("foo", child)
	assert.ErrorIs(t, err, ErrNotDirectory)
}

func TestAddEntryEmptyName(t *testing.T) {
	dir, err := NewDirectory()
	require.NoError(t, err)

	child, err := NewFile("x")
	require.NoError(t, err)

	err = dir.AddEntry("", child)
	assert.ErrorIs(t, err, ErrInvalidName)
}
