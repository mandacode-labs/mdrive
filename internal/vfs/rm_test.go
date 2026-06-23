package vfs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRm(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	_, err := svc.Touch(ctx, "d1", "/x")
	require.NoError(t, err)
	_, err = svc.Touch(ctx, "d1", "/y")
	require.NoError(t, err)

	err = svc.Rm(ctx, "d1", []string{"/x", "/y"}, false)
	require.NoError(t, err)

	_, err = svc.Stat(ctx, "d1", "/x")
	assert.Error(t, err)
}

func TestRmRecursive(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	_, err := svc.Mkdir(ctx, "d1", "/dir")
	require.NoError(t, err)
	_, err = svc.Touch(ctx, "d1", "/dir/a")
	require.NoError(t, err)
	_, err = svc.Touch(ctx, "d1", "/dir/b")
	require.NoError(t, err)

	err = svc.Rm(ctx, "d1", []string{"/dir"}, true)
	require.NoError(t, err)

	_, err = svc.Stat(ctx, "d1", "/dir")
	assert.Error(t, err)
}

func TestRmDirWithoutRecursive(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	_, err := svc.Mkdir(ctx, "d1", "/dir")
	require.NoError(t, err)

	err = svc.Rm(ctx, "d1", []string{"/dir"}, false)
	assert.Error(t, err)
}
