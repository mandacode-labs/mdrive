package gc

import (
	"context"
	"fmt"

	"github.com/mandacode-labs/mdrive/ent"
	entdrivestorage "github.com/mandacode-labs/mdrive/ent/drivestorage"
	entgctombstone "github.com/mandacode-labs/mdrive/ent/gctombstone"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs"
)

// Tombstone is a single pending S3 deletion that has been written
// to the gc_tombstones table. The Bucket/Key describe the S3
// object the gc.TombstoneCleaner job will pass to the S3 client;
// ID is the database row that records the work to be done.
type Tombstone struct {
	ID     int
	Bucket string
	Key    string
}

// GarbageRecorder writes tombstone rows for S3 objects whose
// nodes have been removed. It implements vfs.GarbageRecorder:
// vfs calls RecordGarbage on every unlink whose target was an
// object node.
type GarbageRecorder struct {
	client *ent.Client
}

// NewGarbageRecorder returns a vfs.GarbageRecorder backed by the
// gc_tombstones ent table.
func NewGarbageRecorder(client *ent.Client) *GarbageRecorder {
	return &GarbageRecorder{client: client}
}

// RecordGarbage writes one tombstone row per ref. The writes
// commit independently of the node transaction that deleted the
// source nodes; on failure the caller must accept that the S3
// objects are orphaned (a future orphan-scan job can reclaim
// them).
func (g *GarbageRecorder) RecordGarbage(ctx context.Context, refs []vfs.GarbageRef) error {
	if len(refs) == 0 {
		return nil
	}
	bulk := make([]*ent.GCTombstoneCreate, len(refs))
	for i, r := range refs {
		bulk[i] = g.client.GCTombstone.Create().SetBucket(r.Bucket).SetKey(r.Key)
	}
	_, err := g.client.GCTombstone.CreateBulk(bulk...).Save(ctx)
	if err != nil {
		return errorx.Wrap(err, fmt.Sprintf("gc: tombstone insert (count=%d)", len(refs)))
	}
	return nil
}

// TombstoneGroup is a batch of keys in a single bucket.
type TombstoneGroup struct {
	Bucket string
	Keys   []string
	IDs    []int
}

// QueryTombstones returns up to limit tombstone records grouped by
// bucket. The TombstoneCleaner job processes each group in one
// S3 DeleteObjects call.
func QueryTombstones(ctx context.Context, client *ent.Client, limit int) ([]TombstoneGroup, error) {
	rows, err := client.GCTombstone.Query().
		Order(ent.Asc(entgctombstone.FieldCreateTime)).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("gc: tombstone query (limit=%d)", limit))
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

// DeleteTombstones removes processed tombstone rows by their IDs.
func DeleteTombstones(ctx context.Context, client *ent.Client, ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := client.GCTombstone.Delete().Where(entgctombstone.IDIn(ids...)).Exec(ctx)
	if err != nil {
		return errorx.Wrap(err, fmt.Sprintf("gc: tombstone delete (count=%d)", len(ids)))
	}
	return nil
}

// StorageForBucket returns the S3 client config for the first drive
// using the given bucket. The TombstoneCleaner uses this to construct
// an S3 client per bucket group.
type StorageForBucket struct {
	Region       string
	Endpoint     string
	AccessKey    string
	SecretKey    string
	UsePathStyle bool
}

// FindStorageByBucket returns the S3 client config for the first
// drive using the given bucket.
func FindStorageByBucket(ctx context.Context, client *ent.Client, bucket string) (StorageForBucket, error) {
	s, err := client.DriveStorage.Query().Where(entdrivestorage.BucketEQ(bucket)).First(ctx)
	if err != nil {
		return StorageForBucket{}, errorx.Wrap(err, fmt.Sprintf("gc: storage for bucket (bucket=%s)", bucket))
	}
	var endpoint string
	if s.Endpoint != nil {
		endpoint = *s.Endpoint
	}
	return StorageForBucket{
		Region:       s.Region,
		Endpoint:     endpoint,
		AccessKey:    s.AccessKey,
		SecretKey:    s.SecretKey,
		UsePathStyle: s.UsePathStyle,
	}, nil
}
