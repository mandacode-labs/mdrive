package fs_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mandacode-labs/retrowin-go/internal/application/fs"
)

func TestCreateFileCommand(t *testing.T) {
	cmd := &fs.CreateFileCommand{
		SystemID: "system-123",
		UID:      1000,
		GID:      1000,
		Mode:     0644,
		Flags:    0,
		Content:  []byte("test content"),
	}

	assert.Equal(t, "system-123", cmd.SystemID)
	assert.Equal(t, 1000, cmd.UID)
	assert.Equal(t, 1000, cmd.GID)
	assert.Equal(t, 0644, cmd.Mode)
	assert.Equal(t, 0, cmd.Flags)
	assert.Equal(t, []byte("test content"), cmd.Content)
}

func TestCreateDirectoryCommand(t *testing.T) {
	cmd := &fs.CreateDirectoryCommand{
		SystemID: "system-123",
		UID:      1000,
		GID:      1000,
		Mode:     0755,
		Flags:    0,
	}

	assert.Equal(t, "system-123", cmd.SystemID)
	assert.Equal(t, 1000, cmd.UID)
	assert.Equal(t, 1000, cmd.GID)
	assert.Equal(t, 0755, cmd.Mode)
	assert.Equal(t, 0, cmd.Flags)
}

func TestCreateSymlinkCommand(t *testing.T) {
	cmd := &fs.CreateSymlinkCommand{
		SystemID: "system-123",
		UID:      1000,
		GID:      1000,
		Mode:     0777,
		Flags:    0,
		Target:   "/target/path",
	}

	assert.Equal(t, "system-123", cmd.SystemID)
	assert.Equal(t, 1000, cmd.UID)
	assert.Equal(t, 1000, cmd.GID)
	assert.Equal(t, 0777, cmd.Mode)
	assert.Equal(t, 0, cmd.Flags)
	assert.Equal(t, "/target/path", cmd.Target)
}

func TestUpdateContentCommand(t *testing.T) {
	cmd := &fs.UpdateContentCommand{
		ID:      "inode-123",
		Content: []byte("new content"),
	}

	assert.Equal(t, "inode-123", cmd.ID)
	assert.Equal(t, []byte("new content"), cmd.Content)
}

func TestUpdateModeCommand(t *testing.T) {
	cmd := &fs.UpdateModeCommand{
		ID:   "inode-123",
		Mode: 0755,
	}

	assert.Equal(t, "inode-123", cmd.ID)
	assert.Equal(t, 0755, cmd.Mode)
}

func TestListFilter(t *testing.T) {
	systemID := "system-123"
	uid := 1000

	filter := &fs.ListFilter{
		SystemID: &systemID,
		UID:      &uid,
	}

	assert.Equal(t, "system-123", *filter.SystemID)
	assert.Equal(t, 1000, *filter.UID)
}

func TestRmCommand(t *testing.T) {
	cmd := &fs.RmCommand{
		SystemID: "system-123",
		Paths:    []string{"/file1", "/file2"},
	}

	assert.Equal(t, "system-123", cmd.SystemID)
	assert.Equal(t, []string{"/file1", "/file2"}, cmd.Paths)
}

func TestMvCommand(t *testing.T) {
	cmd := &fs.MvCommand{
		SystemID:    "system-123",
		Sources:     []string{"/file1"},
		Destination: "/dest",
	}

	assert.Equal(t, "system-123", cmd.SystemID)
	assert.Equal(t, []string{"/file1"}, cmd.Sources)
	assert.Equal(t, "/dest", cmd.Destination)
}

func TestRenameCommand(t *testing.T) {
	cmd := &fs.RenameCommand{
		SystemID: "system-123",
		Path:     "/old",
		NewName:  "new",
	}

	assert.Equal(t, "system-123", cmd.SystemID)
	assert.Equal(t, "/old", cmd.Path)
	assert.Equal(t, "new", cmd.NewName)
}
