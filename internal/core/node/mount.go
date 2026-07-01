package node

import (
	"encoding/json"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

var ErrInvalidMountReference = errorx.New(errorx.KindBadRequest, "node: mount source drive id is required")

// MountContent is the JSON-serialized payload of a mount node. A mount
// node represents a bind-style reference to another drive's root
// directory: when path resolution reaches it, the resolver switches
// context to SourceDriveID and continues with the remaining path.
type MountContent struct {
	SourceDriveID string `json:"src"`
}

// NewMountContent creates a MountContent for the given source drive id.
func NewMountContent(sourceDriveID string) *MountContent {
	return &MountContent{SourceDriveID: sourceDriveID}
}

// Marshal returns the JSON representation of MountContent.
func (m *MountContent) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

// NewMount creates a new mount node pointing to sourceDriveID's root.
// The mount node starts with nlink=0 (it has no hardlinks of its own;
// the mount is purely a directory entry, so it is never hardlinked
// away independently).
func NewMount(sourceDriveID string) (*Node, error) {
	if sourceDriveID == "" {
		return nil, ErrInvalidMountReference
	}
	return newInlineNode(NodeTypeMount, NewMountContent(sourceDriveID), 0)
}

// ReadMount returns the source drive id this mount points to.
func (n *Node) ReadMount() (string, error) {
	content, err := n.read()
	if err != nil {
		return "", errorx.Wrap(err, "node: read mount content")
	}
	var mc MountContent
	if err := json.Unmarshal(content, &mc); err != nil {
		return "", errorx.Wrap(err, "node: unmarshal mount content")
	}
	return mc.SourceDriveID, nil
}
