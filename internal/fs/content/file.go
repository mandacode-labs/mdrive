package content

import "encoding/json"

// FileContent is the inline payload of a small file-kind node.
// Stored JSON-marshaled in the node's data field, up to
// MaxDataSize (4KB). Mime/Encoding/Checksum ride along with
// the body so callers don't need a second roundtrip to stat
// the file.
type FileContent struct {
	Raw      string `json:"raw"`
	Mime     string `json:"mime,omitempty"`
	Encoding string `json:"enc,omitempty"`
	Checksum string `json:"sum,omitempty"`
}

// Marshal implements [Content].
func (f *FileContent) Marshal() ([]byte, error) {
	return json.Marshal(f)
}

func NewFileContent(raw string) Content {
	return &FileContent{Raw: raw}
}
