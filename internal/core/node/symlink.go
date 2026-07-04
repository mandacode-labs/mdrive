package node

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// SymlinkContent is the JSON-serialized target of a symlink node.
// Short symlinks are stored inline; very long targets would need an external block,
// but for simplicity we keep the same 4 KiB inline limit as other node types.
type SymlinkContent struct {
	Target uuid.UUID `json:"target"`
}

// NewSymlinkContent creates a SymlinkContent with the given target.
func NewSymlinkContent(target uuid.UUID) *SymlinkContent {
	return &SymlinkContent{Target: target}
}

// Marshal returns the JSON representation of SymlinkContent.
func (s *SymlinkContent) Marshal() ([]byte, error) {
	return json.Marshal(s)
}

// NewSymlink creates a new symlink node pointing to the given target.
func NewSymlink(target uuid.UUID) (*Node, error) {
	if target == uuid.Nil {
		return nil, errorx.New(errorx.KindInvalidArgument, "node: invalid target")
	}
	return newInlineNode(NodeKindSymlink, NewSymlinkContent(target), int64(len(target)))
}

// Readlink returns the symlink's target path (POSIX readlink(2)). The
// caller must have already verified the node is a symlink; calling
// Readlink on a non-symlink node returns ErrInvalidType.
func (n *Node) Readlink() (uuid.UUID, error) {
	if n.kind != NodeKindSymlink {
		return uuid.Nil, errorx.New(errorx.KindInvalidArgument, "node: invalid type for readlink")
	}
	content, err := n.read()
	if err != nil {
		return uuid.Nil, errorx.Wrap(err, "node: read symlink content")
	}
	var sc SymlinkContent
	if err := json.Unmarshal(content, &sc); err != nil {
		return uuid.Nil, errorx.Wrap(err, "node: unmarshal symlink content")
	}
	return sc.Target, nil
}

// WriteSymlink updates the symlink's target.
func (n *Node) WriteSymlink(target uuid.UUID) error {
	if target == uuid.Nil {
		return errorx.New(errorx.KindInvalidArgument, "node: invalid target")
	}
	if n.kind != NodeKindSymlink {
		return errorx.New(errorx.KindInvalidArgument, "node: invalid type for operation")
	}
	data, err := json.Marshal(NewSymlinkContent(target))
	if err != nil {
		return errorx.Wrap(err, "node: marshal symlink content")
	}
	if len(data) > MaxContentSize {
		return errorx.New(errorx.KindInvalidArgument, "node: content exceeds maximum size")
	}
	return n.write(Content(data), int64(len(target)))
}
