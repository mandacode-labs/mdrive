package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/valkey-io/valkey-go"
)

// ValkeyStore implements Store with Valkey.
type ValkeyStore struct {
	client valkey.Client
	prefix string
}

const keyPrefix = "mdrive:session:"

// NewValkeyStore creates a session store backed by Valkey.
func NewValkeyStore(client valkey.Client) *ValkeyStore {
	return &ValkeyStore{client: client, prefix: keyPrefix}
}

func (s *ValkeyStore) key(id string) string {
	return s.prefix + id
}

func (s *ValkeyStore) Create(ctx context.Context, sess *Session) error {
	data, err := sess.Encode()
	if err != nil {
		return err
	}
	ttl := sess.TTL()
	if ttl <= 0 {
		return ErrExpired
	}
	resp := s.client.Do(ctx, s.client.B().Set().Key(s.key(sess.ID)).Value(valkey.BinaryString(data)).ExSeconds(int64(ttl.Seconds())).Build())
	if err := resp.Error(); err != nil {
		return fmt.Errorf("session: valkey set: %w", err)
	}
	return nil
}

func (s *ValkeyStore) Get(ctx context.Context, id string) (*Session, error) {
	resp := s.client.Do(ctx, s.client.B().Get().Key(s.key(id)).Build())
	if err := resp.Error(); err != nil {
		if errors.Is(err, valkey.Nil) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("session: valkey get: %w", err)
	}
	data, err := resp.AsBytes()
	if err != nil {
		return nil, fmt.Errorf("session: valkey as bytes: %w", err)
	}
	sess, err := Decode(data)
	if err != nil {
		return nil, err
	}
	if sess.IsExpired() {
		_ = s.Delete(ctx, id)
		return nil, ErrExpired
	}
	return sess, nil
}

func (s *ValkeyStore) Delete(ctx context.Context, id string) error {
	resp := s.client.Do(ctx, s.client.B().Del().Key(s.key(id)).Build())
	if err := resp.Error(); err != nil {
		return fmt.Errorf("session: valkey del: %w", err)
	}
	return nil
}

// Scan iterates all session keys using the Valkey SCAN command. The
// callback receives the session ID (not the full key). Returning an
// error from fn aborts the scan.
func (s *ValkeyStore) Scan(ctx context.Context, fn func(id string) error) error {
	cursor := uint64(0)
	for {
		resp := s.client.Do(ctx, s.client.B().Scan().Cursor(cursor).Match(s.prefix+"*").Count(100).Build())
		if err := resp.Error(); err != nil {
			return fmt.Errorf("session: valkey scan: %w", err)
		}
		arr, err := resp.AsScanEntry()
		if err != nil {
			return fmt.Errorf("session: valkey scan entry: %w", err)
		}
		for _, k := range arr.Elements {
			id := strings.TrimPrefix(k, s.prefix)
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

var _ Store = (*ValkeyStore)(nil)
var _ Scanner = (*ValkeyStore)(nil)
