package inode_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/retrowin-go/internal/core/inode"
)

func TestInode_NewInode(t *testing.T) {
	now := time.Now()
	content := []byte("test content")

	in := inode.NewInode(
		"inode-123",
		"system-456",
		inode.ModeRegular|0644,
		1000,
		1000,
		12,
		1,
		0,
		now,
		now,
		now,
		content,
		now,
		now,
	)

	require.NotNil(t, in)
	assert.Equal(t, "inode-123", in.ID())
	assert.Equal(t, "system-456", in.SystemID())
	assert.Equal(t, inode.ModeRegular|0644, in.Mode())
	assert.Equal(t, 1000, in.UID())
	assert.Equal(t, 1000, in.GID())
	assert.Equal(t, int64(12), in.Size())
	assert.Equal(t, 1, in.LinkCount())
	assert.Equal(t, 0, in.Flags())
	assert.Equal(t, content, in.Content())
}

func TestInode_FileType(t *testing.T) {
	tests := []struct {
		name     string
		mode     int
		expected int
	}{
		{"regular file", inode.ModeRegular | 0644, inode.ModeRegular},
		{"directory", inode.ModeDirectory | 0755, inode.ModeDirectory},
		{"symlink", inode.ModeSymlink | 0777, inode.ModeSymlink},
		{"object", inode.ModeObject | 0644, inode.ModeObject},
		{"block device", inode.ModeBlock | 0600, inode.ModeBlock},
		{"char device", inode.ModeChar | 0600, inode.ModeChar},
		{"fifo", inode.ModeFifo | 0600, inode.ModeFifo},
		{"socket", inode.ModeSocket | 0755, inode.ModeSocket},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := inode.NewInode(
				"test-id",
				"system-123",
				tt.mode,
				0,
				0,
				0,
				1,
				0,
				time.Now(),
				time.Now(),
				time.Now(),
				nil,
				time.Now(),
				time.Now(),
			)
			assert.Equal(t, tt.expected, in.FileType())
		})
	}
}

func TestInode_Permissions(t *testing.T) {
	tests := []struct {
		name     string
		mode     int
		expected int
	}{
		{"0755", inode.ModeDirectory | 0755, 0755},
		{"0644", inode.ModeRegular | 0644, 0644},
		{"0600", inode.ModeRegular | 0600, 0600},
		{"0777", inode.ModeSymlink | 0777, 0777},
		{"0700", inode.ModeDirectory | 0700, 0700},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := inode.NewInode(
				"test-id",
				"system-123",
				tt.mode,
				0,
				0,
				0,
				1,
				0,
				time.Now(),
				time.Now(),
				time.Now(),
				nil,
				time.Now(),
				time.Now(),
			)
			assert.Equal(t, tt.expected, in.Permissions())
		})
	}
}

func TestInode_IsMethods(t *testing.T) {
	t.Run("IsDir returns true for directory", func(t *testing.T) {
		in := inode.NewInode(
			"test-id",
			"system-123",
			inode.ModeDirectory|0755,
			0, 0, 0, 1, 0,
			time.Now(), time.Now(), time.Now(), nil,
			time.Now(), time.Now(),
		)
		assert.True(t, in.IsDir())
		assert.False(t, in.IsRegular())
		assert.False(t, in.IsSymlink())
		assert.False(t, in.IsObject())
	})

	t.Run("IsRegular returns true for regular file", func(t *testing.T) {
		in := inode.NewInode(
			"test-id",
			"system-123",
			inode.ModeRegular|0644,
			0, 0, 0, 1, 0,
			time.Now(), time.Now(), time.Now(), nil,
			time.Now(), time.Now(),
		)
		assert.False(t, in.IsDir())
		assert.True(t, in.IsRegular())
		assert.False(t, in.IsSymlink())
		assert.False(t, in.IsObject())
	})

	t.Run("IsSymlink returns true for symlink", func(t *testing.T) {
		in := inode.NewInode(
			"test-id",
			"system-123",
			inode.ModeSymlink|0777,
			0, 0, 0, 1, 0,
			time.Now(), time.Now(), time.Now(), nil,
			time.Now(), time.Now(),
		)
		assert.False(t, in.IsDir())
		assert.False(t, in.IsRegular())
		assert.True(t, in.IsSymlink())
		assert.False(t, in.IsObject())
	})

	t.Run("IsObject returns true for object", func(t *testing.T) {
		in := inode.NewInode(
			"test-id",
			"system-123",
			inode.ModeObject|0644,
			0, 0, 0, 1, 0,
			time.Now(), time.Now(), time.Now(), nil,
			time.Now(), time.Now(),
		)
		assert.False(t, in.IsDir())
		assert.False(t, in.IsRegular())
		assert.False(t, in.IsSymlink())
		assert.True(t, in.IsObject())
	})
}

func TestModeConstants(t *testing.T) {
	// Verify file type constants
	assert.Equal(t, 0xF000, inode.ModeTypeMask)
	assert.Equal(t, 0x8000, inode.ModeRegular)
	assert.Equal(t, 0x4000, inode.ModeDirectory)
	assert.Equal(t, 0xA000, inode.ModeSymlink)
	assert.Equal(t, 0x3000, inode.ModeObject)
	assert.Equal(t, 0x6000, inode.ModeBlock)
	assert.Equal(t, 0x2000, inode.ModeChar)
	assert.Equal(t, 0x1000, inode.ModeFifo)
	assert.Equal(t, 0xC000, inode.ModeSocket)

	// Verify permission bits
	assert.Equal(t, 0x0100, inode.PermOwnerRead)
	assert.Equal(t, 0x0080, inode.PermOwnerWrite)
	assert.Equal(t, 0x0040, inode.PermOwnerExec)
	assert.Equal(t, 0x0020, inode.PermGroupRead)
	assert.Equal(t, 0x0010, inode.PermGroupWrite)
	assert.Equal(t, 0x0008, inode.PermGroupExec)
	assert.Equal(t, 0x0004, inode.PermOtherRead)
	assert.Equal(t, 0x0002, inode.PermOtherWrite)
	assert.Equal(t, 0x0001, inode.PermOtherExec)

	// Verify permission combinations
	assert.Equal(t, inode.PermOwnerRead|inode.PermOwnerWrite|inode.PermOwnerExec, inode.PermOwnerRWX)
	assert.Equal(t, inode.PermGroupRead|inode.PermGroupExec, inode.PermGroupRX)
	assert.Equal(t, inode.PermOtherRead|inode.PermOtherExec, inode.PermOtherRX)
	assert.Equal(t, inode.PermOtherRead, inode.PermOtherR)
	assert.Equal(t, inode.PermOwnerRead|inode.PermOwnerWrite, inode.PermOwnerRW)
}

func TestFlagConstants(t *testing.T) {
	// FlagRoot should be 1 (1 << 0)
	assert.Equal(t, 1, inode.FlagRoot)
}

func TestFilterHelpers(t *testing.T) {
	t.Run("ByID creates filter with ID", func(t *testing.T) {
		f := inode.ByID("inode-123")
		require.NotNil(t, f.ID)
		assert.Equal(t, "inode-123", *f.ID)
	})

	t.Run("BySystemID creates filter with SystemID", func(t *testing.T) {
		f := inode.BySystemID("system-456")
		require.NotNil(t, f.SystemID)
		assert.Equal(t, "system-456", *f.SystemID)
	})

	t.Run("ByUID creates filter with UID", func(t *testing.T) {
		uid := 1000
		f := inode.ByUID(uid)
		require.NotNil(t, f.UID)
		assert.Equal(t, uid, *f.UID)
	})
}
