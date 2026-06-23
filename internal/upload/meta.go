package upload

import (
	"encoding/json"
	"fmt"
	"time"
)

// PresignMeta holds server-side state for an in-progress upload.
type PresignMeta struct {
	UploadID    string    `json:"upload_id"`
	DriveID     string    `json:"drive_id"`
	UserID      string    `json:"user_id"`
	Path        string    `json:"path"`
	Bucket      string    `json:"bucket"`
	Key         string    `json:"key"`
	ContentType *string   `json:"content_type,omitempty"`
	Size        *int64    `json:"size,omitempty"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// Encode serializes PresignMeta to JSON bytes.
func (m PresignMeta) Encode() ([]byte, error) {
	return json.Marshal(m)
}

// DecodePresignMeta deserializes JSON bytes into PresignMeta.
func DecodePresignMeta(data []byte) (PresignMeta, error) {
	var m PresignMeta
	if err := json.Unmarshal(data, &m); err != nil {
		return PresignMeta{}, fmt.Errorf("upload: decode meta: %w", err)
	}
	return m, nil
}

// IsExpired reports whether the token has passed its ExpiresAt time.
func (m PresignMeta) IsExpired() bool {
	return time.Now().After(m.ExpiresAt)
}
