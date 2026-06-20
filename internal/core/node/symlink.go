package node

import (
	"encoding/json"
	"fmt"
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
	data, err := json.Marshal(NewSymlinkContent(target))
	if err != nil {
		return nil, fmt.Errorf("failed to marshal symlink content: %w", err)
	}
	if len(data) > MaxContentSize {
		return nil, ErrContentTooLarge
	}
	n := newNode(NodeTypeSymlink)
	if err := n.write(Content(data), int64(len(target))); err != nil {
		return nil, err
	}
	return n, nil
}

// ReadSymlink returns the symlink's target.
func (n *Node) ReadSymlink() (string, error) {
	content, err := n.read()
	if err != nil {
		return "", fmt.Errorf("failed to read symlink content: %w", err)
	}
	var sc SymlinkContent
	if err := json.Unmarshal(content, &sc); err != nil {
		return "", fmt.Errorf("failed to unmarshal symlink content: %w", err)
	}
	return sc.Target, nil
}

// WriteSymlink updates the symlink's target.
func (n *Node) WriteSymlink(target string) error {
	if target == "" {
		return ErrInvalidName
	}
	if n.typ != NodeTypeSymlink {
		return ErrInvalidType
	}
	data, err := json.Marshal(NewSymlinkContent(target))
	if err != nil {
		return fmt.Errorf("failed to marshal symlink content: %w", err)
	}
	if len(data) > MaxContentSize {
		return ErrContentTooLarge
	}
	return n.write(Content(data), int64(len(target)))
}
