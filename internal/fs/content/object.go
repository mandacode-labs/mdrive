package content

import "encoding/json"

// ObjectContent is the inline payload of an object-kind node.
// The actual blob lives in S3; this struct carries the S3
// reference plus the metadata callers need without a second
// roundtrip to the bucket.
type ObjectContent struct {
	Bucket   string `json:"bucket"`
	Key      string `json:"key"`
	Mime     string `json:"mime"`
	Checksum string `json:"sum,omitempty"`
	Size     int64  `json:"size"`
}

func (o *ObjectContent) Marshal() ([]byte, error) {
	return json.Marshal(o)
}

func NewObjectContent(bucket, key, mime, checksum string, size int64) Content {
	return &ObjectContent{
		Bucket:   bucket,
		Key:      key,
		Mime:     mime,
		Checksum: checksum,
		Size:     size,
	}
}
