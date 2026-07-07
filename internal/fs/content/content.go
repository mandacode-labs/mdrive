package content

// Content is the JSON-marshaled inline payload of a node. It
// exists in its own package so fs/node.go (which imports
// errorx) and fs/dir.go can stay free of a circular dep with
// the content payload types.
type Content interface {
	Marshal() ([]byte, error)
}
