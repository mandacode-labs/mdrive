package vfs_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMv(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	_, err := svc.Mkdir(ctx, "d1", "/dst")
	require.NoError(t, err)
	_, err = svc.Touch(ctx, "d1", "/x")
	require.NoError(t, err)

	err = svc.Mv(ctx, "d1", []string{"/x"}, "d1", "/dst/x")
	require.NoError(t, err)

	_, err = svc.Stat(ctx, "d1", "/x")
	assert.Error(t, err)

	_, err = svc.Stat(ctx, "d1", "/dst/x")
	assert.NoError(t, err)
}

func TestMvRejectsCrossDrive(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	_, err := svc.Touch(ctx, "d1", "/x")
	require.NoError(t, err)

	err = svc.Mv(ctx, "d1", []string{"/x"}, "d2", "/x")
	assert.Error(t, err, "cross-drive move not supported")
}
