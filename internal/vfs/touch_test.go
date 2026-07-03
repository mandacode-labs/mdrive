package vfs_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTouch(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	n, err := svc.Touch(ctx, "d1", "/hello.txt")
	require.NoError(t, err)
	assert.True(t, n.IsFile())

	raw, err := svc.Cat(ctx, "d1", "/hello.txt")
	require.NoError(t, err)
	assert.Empty(t, string(raw))
}
