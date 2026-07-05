package content

import "encoding/json"

// FileContent is the JSON-serialized content of a small text file node.
// It is stored inline in the node's content field, up to MaxContentSize.
type FileContent struct {
	Raw string `json:"raw"`
}

// Marshal implements [Content].
func (f *FileContent) Marshal() ([]byte, error) {
	return json.Marshal(f)
}

func NewFileContent(raw string) Content {
	return &FileContent{Raw: raw}
}
