package vfs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStat(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	_, err := svc.Touch(ctx, "user1", "d1", "/x")
	require.NoError(t, err)

	n, err := svc.Stat(ctx, "user1", "d1", "/x")
	require.NoError(t, err)
	assert.True(t, n.IsFile())
	assert.Equal(t, int64(0), n.Size())
}
