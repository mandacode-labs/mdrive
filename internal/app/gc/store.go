package gc

import (
	"context"
	"fmt"

	"github.com/mandacode-labs/mdrive/ent"
	entdrivestorage "github.com/mandacode-labs/mdrive/ent/drivestorage"
	entgctombstone "github.com/mandacode-labs/mdrive/ent/gctombstone"
	"github.com/mandacode-labs/mdrive/internal/entx"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
)

// Tombstone is a single pending S3 deletion.
type Tombstone struct {
	ID     int
	Bucket string
	Key    string
}

// Recorder writes tombstone rows for deleted S3 objects.
// Implements fs.GarbageRecorder.
type Recorder struct {
	client *ent.Client
}

func NewGarbageRecorder(client *ent.Client) *Recorder {
	return &Recorder{client: client}
}

// RecordGarbage writes one tombstone row per ref. When the caller's
// context carries an entx tx, the inserts run inside that tx so
// they commit or roll back with the caller's writes.
func (g *Recorder) RecordGarbage(ctx context.Context, refs []fs.GarbageRef) error {
	if len(refs) == 0 {
		return nil
	}
	client := g.client
	if tx, ok := entx.FromContext(ctx); ok {
		client = tx.Client()
	}
	bulk := make([]*ent.GCTombstoneCreate, len(refs))
	for i, r := range refs {
		bulk[i] = client.GCTombstone.Create().SetBucket(r.Bucket).SetKey(r.Key)
	}
	_, err := client.GCTombstone.CreateBulk(bulk...).Save(ctx)
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
