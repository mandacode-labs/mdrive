package vfs_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStat(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	_, err := svc.Touch(ctx, "d1", "/x")
	require.NoError(t, err)

	n, err := svc.Stat(ctx, "d1", "/x")
	require.NoError(t, err)
	assert.True(t, n.IsFile())
	assert.Equal(t, int64(0), n.Size())
}
