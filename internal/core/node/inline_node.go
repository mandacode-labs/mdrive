package node

import (
	"encoding/json"
	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// newInlineNode constructs a Node of the given type whose inline
// content is the JSON-marshaled payload. Used by type-specific
// constructors (NewFile, NewSymlink, NewObject, NewMount) which
// only differ in (a) the payload type, (b) any pre-marshal
// validation, and (c) the size to record. Centralizing the
// marshaling + size check + node construction removes the
// repeated 5-line pattern that the type-specific New* functions
// would otherwise duplicate.
func newInlineNode(kind NodeType, payload any, size int64) (*Node, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, errorx.Wrap(err, "node: marshal content (kind=%s)", string(kind))
	}
	if len(data) > MaxContentSize {
		return nil, ErrContentTooLarge
	}
	n := newNode(kind)
	if err := n.write(Content(data), size); err != nil {
		return nil, err
	}
	return n, nil
}
