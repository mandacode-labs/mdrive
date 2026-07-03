package vfs_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteAndCat(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	err := svc.Write(ctx, "d1", "/data.txt", "hello world")
	require.NoError(t, err)

	raw, err := svc.Cat(ctx, "d1", "/data.txt")
	require.NoError(t, err)
	assert.Equal(t, "hello world", string(raw))
}
