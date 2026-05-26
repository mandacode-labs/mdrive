package fs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/mandacode-labs/retrowin-go/internal/core/inode"
	"github.com/mandacode-labs/retrowin-go/internal/errors"
)

func TestInode_CheckPerm(t *testing.T) {
	now := time.Now()

	t.Run("root bypass", func(t *testing.T) {
		in := inode.NewInode("id", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
		err := in.CheckPerm(0, []int{}, inode.AccessRead)
		assert.NoError(t, err)
	})

	t.Run("owner has read permission", func(t *testing.T) {
		in := inode.NewInode("id", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
		err := in.CheckPerm(1000, []int{}, inode.AccessRead)
		assert.NoError(t, err)
	})

	t.Run("owner missing write permission", func(t *testing.T) {
		in := inode.NewInode("id", "sys", inode.ModeRegular|0444, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
		err := in.CheckPerm(1000, []int{}, inode.AccessWrite)
		assert.Error(t, err)
		assert.True(t, errors.IsForbidden(err))
	})

	t.Run("group has read permission", func(t *testing.T) {
		in := inode.NewInode("id", "sys", inode.ModeRegular|0640, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
		err := in.CheckPerm(2000, []int{1000}, inode.AccessRead)
		assert.NoError(t, err)
	})

	t.Run("group missing write permission", func(t *testing.T) {
		in := inode.NewInode("id", "sys", inode.ModeRegular|0640, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
		err := in.CheckPerm(2000, []int{1000}, inode.AccessWrite)
		assert.Error(t, err)
		assert.True(t, errors.IsForbidden(err))
	})

	t.Run("other has read permission", func(t *testing.T) {
		in := inode.NewInode("id", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
		err := in.CheckPerm(2000, []int{2000}, inode.AccessRead)
		assert.NoError(t, err)
	})

	t.Run("other missing write permission", func(t *testing.T) {
		in := inode.NewInode("id", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
		err := in.CheckPerm(2000, []int{2000}, inode.AccessWrite)
		assert.Error(t, err)
		assert.True(t, errors.IsForbidden(err))
	})

	t.Run("all permissions", func(t *testing.T) {
		in := inode.NewInode("id", "sys", inode.ModeRegular|0777, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
		err := in.CheckPerm(2000, []int{2000}, inode.AccessExecute)
		assert.NoError(t, err)
	})

	t.Run("no permissions", func(t *testing.T) {
		in := inode.NewInode("id", "sys", inode.ModeRegular, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
		err := in.CheckPerm(2000, []int{2000}, inode.AccessRead)
		assert.Error(t, err)
		assert.True(t, errors.IsForbidden(err))
	})
}

func TestInode_ObjectID(t *testing.T) {
	now := time.Now()

	t.Run("success", func(t *testing.T) {
		objContent := []byte(`{"object_id":"obj-1"}`)
		in := inode.NewInode("id", "sys", inode.ModeObject|0644, 0, 0, 0, 1, 0, now, now, now, objContent, now, now)
		objID, err := in.ObjectID()
		assert.NoError(t, err)
		assert.Equal(t, "obj-1", objID)
	})

	t.Run("not object", func(t *testing.T) {
		in := inode.NewInode("id", "sys", inode.ModeRegular|0644, 0, 0, 0, 1, 0, now, now, now, nil, now, now)
		_, err := in.ObjectID()
		assert.Error(t, err)
		assert.True(t, errors.IsBadRequest(err))
	})

	t.Run("unparsable", func(t *testing.T) {
		in := inode.NewInode("id", "sys", inode.ModeObject|0644, 0, 0, 0, 1, 0, now, now, now, []byte("invalid"), now, now)
		_, err := in.ObjectID()
		assert.Error(t, err)
	})
}
