package upload

import (
	"fmt"
	"context"
	"errors"
	"strings"
	"time"

	"github.com/valkey-io/valkey-go"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// ValkeyRegistry implements TokenRegistry with Valkey (or Redis).
type ValkeyRegistry struct {
	client valkey.Client
	keyFunc  KeyFunc
}

// KeyFunc returns the storage key for an uploadID.
type KeyFunc func(uploadID string) string

const DefaultKeyPrefix = "mdrive:upload:"

// DefaultKey prefixes upload IDs to avoid collisions in shared Valkey instances.
func DefaultKey(uploadID string) string {
	return DefaultKeyPrefix + uploadID
}

// NewValkeyRegistry creates a TokenRegistry backed by Valkey.
func NewValkeyRegistry(client valkey.Client) *ValkeyRegistry {
	return &ValkeyRegistry{client: client, keyFunc: DefaultKey}
}

// NewValkeyRegistryWithKeyer creates a TokenRegistry backed by
// Valkey with a custom keyFunc.
func NewValkeyRegistryWithKeyer(client valkey.Client, keyFunc KeyFunc) *ValkeyRegistry {
	return &ValkeyRegistry{client: client, keyFunc: keyFunc}
}

func (r *ValkeyRegistry) Put(ctx context.Context, meta PresignMeta, ttl time.Duration) error {
	data, err := meta.Encode()
	if err != nil {
		return err
	}
	key := r.keyFunc(meta.UploadID)
	resp := r.client.Do(ctx, r.client.B().Set().Key(key).Value(valkey.BinaryString(data)).ExSeconds(int64(ttl.Seconds())).Build())
	if err := resp.Error(); err != nil {
		return errorx.Wrap(err, fmt.Sprintf("upload: valkey set (upload_id=%s)", meta.UploadID))
	}
	return nil
}

func (r *ValkeyRegistry) Get(ctx context.Context, uploadID string) (PresignMeta, error) {
	key := r.keyFunc(uploadID)
	resp := r.client.Do(ctx, r.client.B().Get().Key(key).Build())
	if err := resp.Error(); err != nil {
		if errors.Is(err, valkey.Nil) {
			return PresignMeta{}, errorx.New(errorx.KindBadRequest, "upload: token not found")
		}
		return PresignMeta{}, errorx.Wrap(err, fmt.Sprintf("upload: valkey get (upload_id=%s)", uploadID))
	}
	data, err := resp.AsBytes()
	if err != nil {
		return PresignMeta{}, errorx.Wrap(err, fmt.Sprintf("upload: valkey as bytes (upload_id=%s)", uploadID))
	}
	meta, err := DecodePresignMeta(data)
	if err != nil {
		return PresignMeta{}, err
	}
	if meta.IsExpired() {
		_ = r.Delete(ctx, uploadID)
		return PresignMeta{}, errorx.New(errorx.KindBadRequest, "upload: token not found")
	}
	return meta, nil
}

func (r *ValkeyRegistry) Delete(ctx context.Context, uploadID string) error {
	key := r.keyFunc(uploadID)
	resp := r.client.Do(ctx, r.client.B().Del().Key(key).Build())
	if err := resp.Error(); err != nil {
		return errorx.Wrap(err, fmt.Sprintf("upload: valkey del (upload_id=%s)", uploadID))
	}
	return nil
}

// Scan iterates all upload keys using the Valkey SCAN command. The
// callback receives the upload ID (not the full key). Returning an
// error from fn aborts the scan.
//
// Scan requires the default keyFunc (DefaultKeyPrefix). Registries
// created with NewValkeyRegistryWithKeyer cannot be scanned here
// because the SCAN MATCH pattern needs a known prefix.
func (r *ValkeyRegistry) Scan(ctx context.Context, fn func(id string) error) error {
	prefix := r.keyFunc("")
	if prefix != DefaultKeyPrefix {
		return errors.New("upload: Scan requires the default keyFunc (use NewValkeyRegistry)")
	}
	cursor := uint64(0)
	for {
		resp := r.client.Do(ctx, r.client.B().Scan().Cursor(cursor).Match(prefix+"*").Count(100).Build())
		if err := resp.Error(); err != nil {
			return errorx.Wrap(err, fmt.Sprintf("upload: valkey scan (prefix=%s)", prefix))
		}
		arr, err := resp.AsScanEntry()
		if err != nil {
			return errorx.Wrap(err, fmt.Sprintf("upload: valkey scan entry (prefix=%s)", prefix))
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

var _ TokenRegistry = (*ValkeyRegistry)(nil)
var _ TokenScanner = (*ValkeyRegistry)(nil)