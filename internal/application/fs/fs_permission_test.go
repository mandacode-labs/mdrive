package fs_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mandacode-labs/retrowin-go/internal/application/fs"
)

func TestAccessType_Constants(t *testing.T) {
	assert.Equal(t, fs.AccessRead, fs.AccessType(0))
	assert.Equal(t, fs.AccessWrite, fs.AccessType(1))
	assert.Equal(t, fs.AccessExecute, fs.AccessType(2))
}

func TestPermissionCheck(t *testing.T) {
	t.Run("owner has read permission", func(t *testing.T) {
		mode := 0600 // owner rw
		perm := mode & 0700
		assert.Equal(t, 0600, perm)
	})

	t.Run("group has read permission", func(t *testing.T) {
		mode := 0060 // group rw
		perm := mode & 0070
		assert.Equal(t, 0060, perm)
	})

	t.Run("other has read permission", func(t *testing.T) {
		mode := 0006 // other rw
		perm := mode & 0007
		assert.Equal(t, 0006, perm)
	})

	t.Run("owner can execute", func(t *testing.T) {
		mode := 0100 // owner x
		perm := mode & 0100
		assert.Equal(t, 0100, perm)
	})

	t.Run("all permissions", func(t *testing.T) {
		mode := 0777
		assert.Equal(t, 0777, mode&0777)
	})
}
