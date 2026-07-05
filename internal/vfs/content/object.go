package content

import "encoding/json"

type ObjectContent struct {
	Bucket   string `json:"bucket"`
	Key      string `json:"key"`
	Mime     string `json:"mime"`
	Checksum string `json:"sum,omitempty"`
}

func (o *ObjectContent) Marshal() ([]byte, error) {
	return json.Marshal(o)
}

func NewObjectContent(bucket, key, mime, checksum string) Content {
	return &ObjectContent{
		Bucket:   bucket,
		Key:      key,
		Mime:     mime,
		Checksum: checksum,
	}
}
