package node

import (
	"encoding/json"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// FileContent is the JSON-serialized content of a small text file node.
// It is stored inline in the node's content field, up to MaxContentSize.
type FileContent struct {
	Raw string `json:"raw"`
}

// NewFileContent creates a new FileContent.
func NewFileContent(raw string) *FileContent {
	return &FileContent{Raw: raw}
}

// Marshal returns the JSON representation of FileContent.
func (f *FileContent) Marshal() ([]byte, error) {
	return json.Marshal(f)
}

// MarshalJSON implements custom marshaling: encodes as a plain string.
func (f *FileContent) MarshalJSON() ([]byte, error) {
	return json.Marshal(f.Raw)
}

// UnmarshalJSON implements custom unmarshaling: decodes a plain string.
func (f *FileContent) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &f.Raw)
}

// NewFile creates a new file node with the given raw text content.
func NewFile(raw string) (*Node, error) {
	return newInlineNode(NodeKindFile, NewFileContent(raw), int64(len(raw)))
}

// WriteFile replaces the file node's content with the given raw text.
func (n *Node) WriteFile(raw string) error {
	data, err := json.Marshal(NewFileContent(raw))
	if err != nil {
		return errorx.Wrap(err, "node: marshal file content")
	}
	if len(data) > MaxContentSize {
		return errorx.New(errorx.KindOutOfRange, "node: content exceeds maximum size")
	}
	return n.write(Content(data), int64(len(raw)))
}

// ReadFile returns the file node's raw text content.
func (n *Node) ReadFile() (string, error) {
	content, err := n.read()
	if err != nil {
		return "", errorx.Wrap(err, "node: read file content")
	}
	var fc FileContent
	if err := json.Unmarshal(content, &fc); err != nil {
		return "", errorx.Wrap(err, "node: unmarshal file content")
	}
	return fc.Raw, nil
}
