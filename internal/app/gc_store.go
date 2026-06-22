package app

import (
	"context"
	"fmt"

	"github.com/mandacode-labs/mdrive/ent"
	entdrivestorage "github.com/mandacode-labs/mdrive/ent/drivestorage"
	entgctombstone "github.com/mandacode-labs/mdrive/ent/gctombstone"

	"github.com/mandacode-labs/mdrive/internal/storage/s3"
	"github.com/mandacode-labs/mdrive/internal/vfs"
)

// gcClient implements vfs.TombstoneInserter using the ent client.
type gcClient struct {
	client *ent.Client
}

func newTombstoneInserter(client *ent.Client) vfs.TombstoneInserter {
	return &gcClient{client: client}
}

// InsertTombstones writes tombstone records for S3 objects whose nodes have been deleted.
func (g *gcClient) InsertTombstones(ctx context.Context, refs []vfs.ObjectRef) error {
	if len(refs) == 0 {
		return nil
	}
	bulk := make([]*ent.GCTombstoneCreate, len(refs))
	for i, r := range refs {
		bulk[i] = g.client.GCTombstone.Create().SetBucket(r.Bucket).SetKey(r.Key)
	}
	_, err := g.client.GCTombstone.CreateBulk(bulk...).Save(ctx)
	if err != nil {
		return fmt.Errorf("gc: insert tombstones: %w", err)
	}
	return nil
}

// InsertTombstonesTx is the transactional variant. It accepts an
// *ent.Tx and writes tombstone rows within the caller's transaction so
// the gctombstone updates commit or roll back atomically with the node
// changes that produced them.
func (g *gcClient) InsertTombstonesTx(ctx context.Context, tx any, refs []vfs.ObjectRef) error {
	if len(refs) == 0 {
		return nil
	}
	entTx, ok := tx.(*ent.Tx)
	if !ok {
		return fmt.Errorf("gc: tombstone tx requires *ent.Tx, got %T", tx)
	}
	bulk := make([]*ent.GCTombstoneCreate, len(refs))
	for i, r := range refs {
		bulk[i] = entTx.GCTombstone.Create().SetBucket(r.Bucket).SetKey(r.Key)
	}
	_, err := entTx.GCTombstone.CreateBulk(bulk...).Save(ctx)
	if err != nil {
		return fmt.Errorf("gc: insert tombstones (tx): %w", err)
	}
	return nil
}

// TombstoneGroup is a batch of keys in a single bucket.
type TombstoneGroup struct {
	Bucket string
	Keys   []string
	IDs    []int
}

// QueryTombstones returns up to limit tombstone records grouped by bucket.
func QueryTombstones(ctx context.Context, client *ent.Client, limit int) ([]TombstoneGroup, error) {
	rows, err := client.GCTombstone.Query().
		Order(ent.Asc(entgctombstone.FieldCreateTime)).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("gc: query tombstones: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}

	byBucket := map[string]TombstoneGroup{}
	for _, r := range rows {
		g := byBucket[r.Bucket]
		g.Bucket = r.Bucket
		g.Keys = append(g.Keys, r.Key)
		g.IDs = append(g.IDs, r.ID)
		byBucket[r.Bucket] = g
	}

	groups := make([]TombstoneGroup, 0, len(byBucket))
	for _, g := range byBucket {
		groups = append(groups, g)
	}
	return groups, nil
}

// DeleteTombstones removes processed tombstone records by their IDs.
func DeleteTombstones(ctx context.Context, client *ent.Client, ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := client.GCTombstone.Delete().Where(entgctombstone.IDIn(ids...)).Exec(ctx)
	if err != nil {
		return fmt.Errorf("gc: delete tombstones: %w", err)
	}
	return nil
}

// FindStorageByBucket returns the S3 client config for the first drive using the given bucket.
func FindStorageByBucket(ctx context.Context, client *ent.Client, bucket string) (s3.Config, error) {
	s, err := client.DriveStorage.Query().Where(entdrivestorage.BucketEQ(bucket)).First(ctx)
	if err != nil {
		return s3.Config{}, fmt.Errorf("gc: storage for bucket %s: %w", bucket, err)
	}
	endpoint := ""
	if s.Endpoint != nil {
		endpoint = *s.Endpoint
	}
	return s3.Config{
		Region:       s.Region,
		Endpoint:     &endpoint,
		AccessKey:    s.AccessKey,
		SecretKey:    s.SecretKey,
		UsePathStyle: s.UsePathStyle,
	}, nil
}
