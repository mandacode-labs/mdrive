// Package upload provides an upload-token registry used by the presigned-upload flow.
package upload

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Common errors.
var (
	ErrNotFound    = errors.New("upload: token not found")
	ErrTokenExists = errors.New("upload: token already exists")
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

// Registry stores and retrieves upload tokens with a TTL.
type Registry interface {
	Put(ctx context.Context, meta PresignMeta, ttl time.Duration) error
	Get(ctx context.Context, uploadID string) (PresignMeta, error)
	Delete(ctx context.Context, uploadID string) error
}

// Scanner is an optional capability of a Registry. Backends that support
// iteration (e.g. Valkey/Redis) implement it so the GC worker can find
// expired tokens to clean up. Backends that do not (e.g. in-memory) can
// omit it; the GC will log and skip.
type Scanner interface {
	Scan(ctx context.Context, fn func(id string) error) error
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
