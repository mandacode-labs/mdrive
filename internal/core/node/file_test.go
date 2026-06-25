package node

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFile(t *testing.T) {
	n, err := NewFile("hello world")
	require.NoError(t, err)

	assert.Equal(t, NodeTypeFile, n.Type())
	assert.Equal(t, int64(len("hello world")), n.Size())
	assert.Equal(t, uint32(0), n.NLink())
	assert.False(t, n.Revision().IsEmpty())
}

func TestNewFileEmpty(t *testing.T) {
	n, err := NewFile("")
	require.NoError(t, err)
	assert.Equal(t, int64(0), n.Size())
}

func TestNewFileTooLarge(t *testing.T) {
	large := make([]byte, MaxContentSize+1)
	for i := range large {
		large[i] = 'a'
	}
	_, err := NewFile(string(large))
	assert.Error(t, err)
}

func TestFileReadWrite(t *testing.T) {
	n, err := NewFile("initial content")
	require.NoError(t, err)
	initialRev := n.Revision()

	err = n.WriteFile("updated content")
	require.NoError(t, err)

	got, err := n.ReadFile()
	require.NoError(t, err)
	assert.Equal(t, "updated content", got)
	assert.NotEqual(t, initialRev, n.Revision())
}
