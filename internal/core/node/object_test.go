package node

import (
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewObject(t *testing.T) {
	oc := NewObjectContent("my-bucket", "path/to/key", "text/plain", "abc123")
	n, err := NewObject(*oc, 1024)
	require.NoError(t, err)

	assert.Equal(t, NodeKindObject, n.Kind())
	assert.Equal(t, int64(1024), n.Size())

	got, err := n.ReadObject()
	require.NoError(t, err)
	assert.Equal(t, "my-bucket", got.Bucket)
	assert.Equal(t, "path/to/key", got.Key)
	assert.Equal(t, "text/plain", got.Mime)
	assert.Equal(t, "abc123", got.Checksum)
}

func TestNewObjectInvalidRef(t *testing.T) {
	_, err := NewObject(ObjectContent{Bucket: "", Key: "k"}, 100)
	assert.True(t, errorx.IsKind(err, errorx.KindInvalidArgument))

	_, err = NewObject(ObjectContent{Bucket: "b", Key: ""}, 100)
	assert.True(t, errorx.IsKind(err, errorx.KindInvalidArgument))
}

func TestNewObjectNegativeSize(t *testing.T) {
	oc := NewObjectContent("b", "k", "text/plain", "")
	_, err := NewObject(*oc, -1)
	assert.True(t, errorx.IsKind(err, errorx.KindInvalidArgument))
}
