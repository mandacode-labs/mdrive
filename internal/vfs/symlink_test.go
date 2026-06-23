package vfs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSymlink(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	_, err := svc.Touch(ctx, "d1", "/target")
	require.NoError(t, err)

	n, err := svc.Symlink(ctx, "d1", "/target", "/link")
	require.NoError(t, err)
	assert.True(t, n.IsSymlink())

	target, err := n.ReadSymlink()
	require.NoError(t, err)
	assert.Equal(t, "/target", target)

	raw, err := svc.Cat(ctx, "d1", "/link")
	require.NoError(t, err)
	assert.Equal(t, "/target", string(raw))
}
