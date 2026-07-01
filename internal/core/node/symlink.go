package node

import (
	"encoding/json"
	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// SymlinkContent is the JSON-serialized target of a symlink node.
// Short symlinks are stored inline; very long targets would need an external block,
// but for simplicity we keep the same 4 KiB inline limit as other node types.
type SymlinkContent struct {
	Target string `json:"target"`
}

// NewSymlinkContent creates a SymlinkContent with the given target.
func NewSymlinkContent(target string) *SymlinkContent {
	return &SymlinkContent{Target: target}
}

// Marshal returns the JSON representation of SymlinkContent.
func (s *SymlinkContent) Marshal() ([]byte, error) {
	return json.Marshal(s)
}

// NewSymlink creates a new symlink node pointing to the given target.
func NewSymlink(target string) (*Node, error) {
	if target == "" {
		return nil, ErrInvalidName
	}
	return newInlineNode(NodeTypeSymlink, NewSymlinkContent(target), int64(len(target)))
}

// Readlink returns the symlink's target path (POSIX readlink(2)). The
// caller must have already verified the node is a symlink; calling
// Readlink on a non-symlink node returns ErrInvalidType.
func (n *Node) Readlink() (string, error) {
	if n.kind != NodeTypeSymlink {
		return "", ErrInvalidType
	}
	content, err := n.read()
	if err != nil {
		return "", errorx.Wrap(err, "node: read symlink content")
	}
	var sc SymlinkContent
	if err := json.Unmarshal(content, &sc); err != nil {
		return "", errorx.Wrap(err, "node: unmarshal symlink content")
	}
	return sc.Target, nil
}

// WriteSymlink updates the symlink's target.
func (n *Node) WriteSymlink(target string) error {
	if target == "" {
		return ErrInvalidName
	}
	if n.kind != NodeTypeSymlink {
		return ErrInvalidType
	}
	data, err := json.Marshal(NewSymlinkContent(target))
	if err != nil {
		return errorx.Wrap(err, "node: marshal symlink content")
	}
	if len(data) > MaxContentSize {
		return ErrContentTooLarge
	}
	return n.write(Content(data), int64(len(target)))
}
