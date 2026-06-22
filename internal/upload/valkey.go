package upload

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/valkey-io/valkey-go"
)

// ValkeyRegistry implements Registry with Valkey (or Redis).
type ValkeyRegistry struct {
	client valkey.Client
	keyer  KeyFunc
}

// KeyFunc returns the storage key for an uploadID.
type KeyFunc func(uploadID string) string

const DefaultKeyPrefix = "mdrive:upload:"

// DefaultKey prefixes upload IDs to avoid collisions in shared Valkey instances.
func DefaultKey(uploadID string) string {
	return DefaultKeyPrefix + uploadID
}

// NewValkeyRegistry creates a Registry backed by Valkey.
func NewValkeyRegistry(client valkey.Client) *ValkeyRegistry {
	return &ValkeyRegistry{client: client, keyer: DefaultKey}
}

// NewValkeyRegistryWithKeyer creates a Registry backed by Valkey with a custom keyer.
func NewValkeyRegistryWithKeyer(client valkey.Client, keyer KeyFunc) *ValkeyRegistry {
	return &ValkeyRegistry{client: client, keyer: keyer}
}

func (r *ValkeyRegistry) Put(ctx context.Context, meta PresignMeta, ttl time.Duration) error {
	data, err := meta.Encode()
	if err != nil {
		return err
	}
	key := r.keyer(meta.UploadID)
	resp := r.client.Do(ctx, r.client.B().Set().Key(key).Value(valkey.BinaryString(data)).ExSeconds(int64(ttl.Seconds())).Build())
	if err := resp.Error(); err != nil {
		return fmt.Errorf("upload: valkey set: %w", err)
	}
	return nil
}

func (r *ValkeyRegistry) Get(ctx context.Context, uploadID string) (PresignMeta, error) {
	key := r.keyer(uploadID)
	resp := r.client.Do(ctx, r.client.B().Get().Key(key).Build())
	if err := resp.Error(); err != nil {
		if errors.Is(err, valkey.Nil) {
			return PresignMeta{}, ErrNotFound
		}
		return PresignMeta{}, fmt.Errorf("upload: valkey get: %w", err)
	}
	data, err := resp.AsBytes()
	if err != nil {
		return PresignMeta{}, fmt.Errorf("upload: valkey as bytes: %w", err)
	}
	meta, err := DecodePresignMeta(data)
	if err != nil {
		return PresignMeta{}, err
	}
	if meta.IsExpired() {
		_ = r.Delete(ctx, uploadID)
		return PresignMeta{}, ErrNotFound
	}
	return meta, nil
}

func (r *ValkeyRegistry) Delete(ctx context.Context, uploadID string) error {
	key := r.keyer(uploadID)
	resp := r.client.Do(ctx, r.client.B().Del().Key(key).Build())
	if err := resp.Error(); err != nil {
		return fmt.Errorf("upload: valkey del: %w", err)
	}
	return nil
}

// Scan iterates all upload keys using the Valkey SCAN command. The
// callback receives the upload ID (not the full key). Returning an
// error from fn aborts the scan.
func (r *ValkeyRegistry) Scan(ctx context.Context, fn func(id string) error) error {
	prefix := DefaultKeyPrefix
	cursor := uint64(0)
	for {
		resp := r.client.Do(ctx, r.client.B().Scan().Cursor(cursor).Match(prefix+"*").Count(100).Build())
		if err := resp.Error(); err != nil {
			return fmt.Errorf("upload: valkey scan: %w", err)
		}
		arr, err := resp.AsScanEntry()
		if err != nil {
			return fmt.Errorf("upload: valkey scan entry: %w", err)
		}
		for _, k := range arr.Elements {
			id := strings.TrimPrefix(k, prefix)
			if err := fn(id); err != nil {
				return err
			}
		}
		cursor = arr.Cursor
		if cursor == 0 {
			return nil
		}
	}
}

var _ Registry = (*ValkeyRegistry)(nil)
var _ Scanner = (*ValkeyRegistry)(nil)
