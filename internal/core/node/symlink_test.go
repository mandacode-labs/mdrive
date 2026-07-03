package node

import (
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSymlink(t *testing.T) {
	n, err := NewSymlink("/target/path")
	require.NoError(t, err)

	assert.Equal(t, NodeKindSymlink, n.Kind())

	target, err := n.Readlink()
	require.NoError(t, err)
	assert.Equal(t, "/target/path", target)
}

func TestNewSymlinkEmpty(t *testing.T) {
	_, err := NewSymlink("")
	assert.True(t, errorx.IsKind(err, errorx.KindBadRequest))
}

func TestSymlinkUpdate(t *testing.T) {
	n, err := NewSymlink("/old")
	require.NoError(t, err)

	err = n.WriteSymlink("/new")
	require.NoError(t, err)

	target, err := n.Readlink()
	require.NoError(t, err)
	assert.Equal(t, "/new", target)
}

func TestReadlinkRejectsNonSymlink(t *testing.T) {
	f, err := NewFile("content")
	require.NoError(t, err)
	_, err = f.Readlink()
	assert.True(t, errorx.IsKind(err, errorx.KindBadRequest))
}
