package node

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSymlink(t *testing.T) {
	n, err := NewSymlink("/target/path")
	require.NoError(t, err)

	assert.Equal(t, NodeTypeSymlink, n.Type())

	target, err := n.ReadSymlink()
	require.NoError(t, err)
	assert.Equal(t, "/target/path", target)
}

func TestNewSymlink_Empty(t *testing.T) {
	_, err := NewSymlink("")
	assert.ErrorIs(t, err, ErrInvalidName)
}

func TestSymlinkUpdate(t *testing.T) {
	n, err := NewSymlink("/old")
	require.NoError(t, err)

	err = n.WriteSymlink("/new")
	require.NoError(t, err)

	target, err := n.ReadSymlink()
	require.NoError(t, err)
	assert.Equal(t, "/new", target)
}
