package node

import (
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDirContentJSONTags(t *testing.T) {
	u := uuidFromString(t, "550e8400-e29b-41d4-a716-446655440000")
	dc := DirContent{
		Entries: []DirEntry{
			{InodeID: u, Name: "x", Type: NodeTypeFile},
		},
	}
	data, err := json.Marshal(dc)
	require.NoError(t, err)

	assert.True(t, contains(data, []byte(`"ino":`)))
	assert.False(t, contains(data, []byte(`"inode_id"`)))
	assert.True(t, contains(data, []byte(`"items":`)))
}

func TestFileContentJSONTag(t *testing.T) {
	fc := FileContent{Raw: "hello"}
	data, err := json.Marshal(fc)
	require.NoError(t, err)
	assert.True(t, contains(data, []byte(`"raw":`)))
}

func TestSymlinkContentJSONTag(t *testing.T) {
	sc := SymlinkContent{Target: "/path"}
	data, err := json.Marshal(sc)
	require.NoError(t, err)
	assert.True(t, contains(data, []byte(`"target":`)))
}

func TestObjectContentJSONTags(t *testing.T) {
	oc := ObjectContent{
		Bucket:   "b",
		Key:      "k",
		Mime:     "text/plain",
		Checksum: "abc",
	}
	data, err := json.Marshal(oc)
	require.NoError(t, err)

	assert.True(t, contains(data, []byte(`"mime":`)))
	assert.True(t, contains(data, []byte(`"sum":`)))
	assert.False(t, contains(data, []byte(`"content_type"`)))
	assert.False(t, contains(data, []byte(`"checksum"`)))
}

func TestContentSize(t *testing.T) {
	c := Content([]byte("hello"))
	assert.Equal(t, 5, c.Size())
}

func TestWriteContentTooLarge(t *testing.T) {
	n, err := NewFile("x")
	require.NoError(t, err)

	large := make([]byte, MaxContentSize+1)
	err = n.write(large, int64(len(large)))
	assertKind(t, err, errorx.KindBadRequest)
}

func TestNewRootNode(t *testing.T) {
	n := NewRootNode()
	assert.Equal(t, NodeTypeDirectory, n.Type())
}

func TestRevision(t *testing.T) {
	r1 := newRevision()
	r2 := r1.Next()
	assert.NotEqual(t, r1, r2)
	assert.False(t, r1.IsEmpty())

	empty := Revision("")
	assert.True(t, empty.IsEmpty())
}

func TestFlags(t *testing.T) {
	var f Flags
	assert.False(t, f.Has(FlagImmutable))

	f.Set(FlagImmutable)
	assert.True(t, f.Has(FlagImmutable))

	f.Set(FlagNoAtime)
	assert.True(t, f.Has(FlagImmutable))
	assert.True(t, f.Has(FlagNoAtime))

	f.Clear(FlagImmutable)
	assert.False(t, f.Has(FlagImmutable))
	assert.True(t, f.Has(FlagNoAtime))
}

func contains(haystack, needle []byte) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if string(haystack[i:i+len(needle)]) == string(needle) {
			return true
		}
	}
	return false
}

func uuidFromString(t *testing.T, s string) uuid.UUID {
	t.Helper()
	u, err := uuid.Parse(s)
	require.NoError(t, err)
	return u
}
