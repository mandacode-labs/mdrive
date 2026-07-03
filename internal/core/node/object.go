package node

import (
	"encoding/json"
	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// ObjectContent is the JSON-serialized reference to externally-stored data (e.g., S3).
// The actual data lives in external object storage; only the reference is inline.
// Size is stored on the Node itself (Node.Size), not in ObjectContent, so that
// callers can observe the file size without decoding the reference.
//
// JSON tags are kept short to minimize inline content size.
type ObjectContent struct {
	Bucket   string `json:"bucket"`
	Key      string `json:"key"`
	Mime     string `json:"mime"`
	Checksum string `json:"sum,omitempty"`
}

// NewObjectContent creates an ObjectContent with the given reference.
func NewObjectContent(bucket, key, mime, checksum string) *ObjectContent {
	return &ObjectContent{
		Bucket:   bucket,
		Key:      key,
		Mime:     mime,
		Checksum: checksum,
	}
}

// Marshal returns the JSON representation of ObjectContent.
func (o *ObjectContent) Marshal() ([]byte, error) {
	return json.Marshal(o)
}

// NewObject creates a new object node referring to data of the given size in external storage.
func NewObject(content ObjectContent, size int64) (*Node, error) {
	if content.Bucket == "" || content.Key == "" {
		return nil, errorx.New(errorx.KindBadRequest, "node: invalid object reference")
	}
	if size < 0 {
		return nil, errorx.New(errorx.KindBadRequest, "node: invalid size")
	}
	return newInlineNode(NodeKindObject, &content, size)
}

// ReadObject returns the object node's external reference.
func (n *Node) ReadObject() (ObjectContent, error) {
	content, err := n.read()
	if err != nil {
		return ObjectContent{}, errorx.Wrap(err, "node: read object content")
	}
	var oc ObjectContent
	if err := json.Unmarshal(content, &oc); err != nil {
		return ObjectContent{}, errorx.Wrap(err, "node: unmarshal object content")
	}
	return oc, nil
}

// WriteObject updates the object node's external reference and size.
func (n *Node) WriteObject(content ObjectContent, size int64) error {
	if n.kind != NodeKindObject {
		return errorx.New(errorx.KindBadRequest, "node: invalid type for operation")
	}
	if content.Bucket == "" || content.Key == "" {
		return errorx.New(errorx.KindBadRequest, "node: invalid object reference")
	}
	if size < 0 {
		return errorx.New(errorx.KindBadRequest, "node: invalid size")
	}
	data, err := json.Marshal(&content)
	if err != nil {
		return errorx.Wrap(err, "node: marshal object content")
	}
	if len(data) > MaxContentSize {
		return errorx.New(errorx.KindBadRequest, "node: content exceeds maximum size")
	}
	return n.write(Content(data), size)
}
