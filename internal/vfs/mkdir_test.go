package vfs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMkdir(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	n, err := svc.Mkdir(ctx, "d1", "/foo")
	require.NoError(t, err)
	assert.True(t, n.IsDir())

	dc, err := svc.Ls(ctx, "d1", "/")
	require.NoError(t, err)
	require.Len(t, dc.Entries, 1)
	assert.Equal(t, "foo", dc.Entries[0].Name)
}

func TestMkdirNested(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	_, err := svc.Mkdir(ctx, "d1", "/a")
	require.NoError(t, err)
	_, err = svc.Mkdir(ctx, "d1", "/a/b")
	require.NoError(t, err)

	dc, err := svc.Ls(ctx, "d1", "/a")
	require.NoError(t, err)
	require.Len(t, dc.Entries, 1)
	assert.Equal(t, "b", dc.Entries[0].Name)
}
