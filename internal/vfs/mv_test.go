package vfs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMv(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	_, err := svc.Mkdir(ctx, "user1", "d1", "/dst")
	require.NoError(t, err)
	_, err = svc.Touch(ctx, "user1", "d1", "/x")
	require.NoError(t, err)

	err = svc.Mv(ctx, "user1", "d1", []string{"/x"}, "d1", "/dst/x")
	require.NoError(t, err)

	_, err = svc.Stat(ctx, "user1", "d1", "/x")
	assert.Error(t, err)

	_, err = svc.Stat(ctx, "user1", "d1", "/dst/x")
	assert.NoError(t, err)
}

func TestMvOverwriteExistingFile(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	_, err := svc.Touch(ctx, "user1", "d1", "/a")
	require.NoError(t, err)
	_, err = svc.Touch(ctx, "user1", "d1", "/b")
	require.NoError(t, err)

	err = svc.Mv(ctx, "user1", "d1", []string{"/a"}, "d1", "/b")
	require.NoError(t, err)

	_, err = svc.Stat(ctx, "user1", "d1", "/a")
	assert.Error(t, err, "source should be gone")

	n, err := svc.Stat(ctx, "user1", "d1", "/b")
	require.NoError(t, err)
	assert.True(t, n.IsFile(), "target should still be a file")
}

func TestMvOverwriteDirFails(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	_, err := svc.Touch(ctx, "user1", "d1", "/a")
	require.NoError(t, err)
	_, err = svc.Mkdir(ctx, "user1", "d1", "/b")
	require.NoError(t, err)

	err = svc.Mv(ctx, "user1", "d1", []string{"/a"}, "d1", "/b")
	assert.Error(t, err, "cannot overwrite directory")
}
