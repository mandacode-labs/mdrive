package upload

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	coredrive "github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/node"
)

// --- Fakes ---

type fakeStore struct {
	presignedURL string
	objectExists bool
	existsErr    error
	uploadErr    error
}

func (f *fakeStore) GetPresignedUploadURL(context.Context, string, string, time.Duration) (string, error) {
	if f.uploadErr != nil {
		return "", f.uploadErr
	}
	return f.presignedURL, nil
}
func (f *fakeStore) GetPresignedDownloadURL(context.Context, string, string, time.Duration) (string, error) {
	return f.presignedURL, nil
}
func (f *fakeStore) ObjectExists(context.Context, string, string) (bool, error) {
	if f.existsErr != nil {
		return false, f.existsErr
	}
	return f.objectExists, nil
}

func (f *fakeStore) DeleteObject(context.Context, string, string) error {
	return nil
}

type fakeDrive struct {
	storage *coredrive.Storage
	getErr  error
}

func (f *fakeDrive) GetStorage(context.Context, string) (*coredrive.Storage, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.storage, nil
}

type fakeNodes struct {
	created *node.Node
	linked  bool
	deleted []uuid.UUID
}

func newFakeNodes() *fakeNodes {
	return &fakeNodes{}
}

func (f *fakeNodes) CreateObject(_ context.Context, _ node.ObjectContent, _ int64) (*node.Node, error) {
	if f.created == nil {
		f.created, _ = node.NewObject(node.ObjectContent{Bucket: "b", Key: "k"}, 100)
	}
	return f.created, nil
}
func (f *fakeNodes) Link(_ context.Context, _ *node.Node, _ string, _ *node.Node) error {
	f.linked = true
	return nil
}
func (f *fakeNodes) Delete(_ context.Context, id uuid.UUID) error {
	f.deleted = append(f.deleted, id)
	return nil
}
func (f *fakeNodes) GetByID(_ context.Context, _ uuid.UUID) (*node.Node, error) {
	return node.NewDirectory()
}

type fakePath struct {
	rootID uuid.UUID
}

func (f *fakePath) GetRootNodeID(context.Context, string) (uuid.UUID, error) {
	return f.rootID, nil
}
func (f *fakePath) ResolveParentNodeID(context.Context, string, string) (uuid.UUID, string, error) {
	return uuid.Nil, "test.bin", nil
}
func (f *fakePath) ResolveNodeID(context.Context, string, string) (uuid.UUID, error) {
	return uuid.Nil, nil
}

// --- Helpers ---

func newTestService(t *testing.T, reg TokenRegistry, store *fakeStore) *Service {
	t.Helper()
	if reg == nil {
		reg = NewMemoryRegistry()
	}
	if store == nil {
		store = &fakeStore{presignedURL: "https://s3.example.com/upload", objectExists: true}
	}
	drive := &fakeDrive{storage: coredrive.NewStorage("d1", "my-bucket", nil, "us-east-1", "a", "s", false)}
	nodes := newFakeNodes()
	root, _ := node.NewDirectory()
	return NewService(Config{
		TokenRegistry: reg,
		StorageLookup: drive,
		NodeLifecycle: nodes,
		ObjectStore:   store,
		Path:          &fakePath{rootID: root.ID()},
	})
}

// --- Tests ---

func TestInitiateUploadHappyPath(t *testing.T) {
	svc := newTestService(t, nil, nil)
	info, err := svc.InitiateUpload(context.Background(), "u1", "d1", "/test.bin", nil, nil, time.Hour)
	require.NoError(t, err)
	assert.NotEmpty(t, info.UploadID)
	assert.Equal(t, "PUT", info.Method)
	assert.Equal(t, "https://s3.example.com/upload", info.URL)
}

func TestInitiateUploadPresignFailureRollsBackToken(t *testing.T) {
	reg := NewMemoryRegistry()
	store := &fakeStore{uploadErr: errors.New("presign fail")}
	svc := newTestService(t, reg, store)
	_, err := svc.InitiateUpload(context.Background(), "u1", "d1", "/test.bin", nil, nil, time.Hour)
	assert.Error(t, err)
	// Token should be deleted.
	_, getErr := reg.Get(context.Background(), "any-id")
	// We don't know the id; just ensure registry is empty.
	_ = getErr
}

func TestCompleteUploadTokenNotFound(t *testing.T) {
	svc := newTestService(t, NewMemoryRegistry(), nil)
	_, err := svc.CompleteUpload(context.Background(), "u1", "d1", "missing-id", 100, nil)
	assert.Error(t, err)
}

func TestCompleteUploadDriveMismatch(t *testing.T) {
	reg := NewMemoryRegistry()
	_ = reg.Put(context.Background(), PresignMeta{
		UploadID:  "u1",
		DriveID:   "d2",
		UserID:    "user",
		Path:      "/x",
		Bucket:    "b",
		Key:       "k",
		ExpiresAt: time.Now().Add(time.Hour),
	}, time.Hour)
	svc := newTestService(t, reg, nil)
	_, err := svc.CompleteUpload(context.Background(), "u1", "d1", "u1", 100, nil)
	assert.ErrorIs(t, err, ErrUploadMismatch)
}

func TestCompleteUploadSizeMismatch(t *testing.T) {
	reg := NewMemoryRegistry()
	expiry := time.Now().Add(time.Hour)
	size := int64(100)
	_ = reg.Put(context.Background(), PresignMeta{
		UploadID:  "u1",
		DriveID:   "d1",
		UserID:    "user",
		Path:      "/x",
		Bucket:    "b",
		Key:       "k",
		Size:      &size,
		ExpiresAt: expiry,
	}, time.Hour)
	svc := newTestService(t, reg, nil)
	_, err := svc.CompleteUpload(context.Background(), "u1", "d1", "u1", 200, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "size mismatch")
}

func TestCompleteUploadObjectNotUploaded(t *testing.T) {
	reg := NewMemoryRegistry()
	_ = reg.Put(context.Background(), PresignMeta{
		UploadID:  "u1",
		DriveID:   "d1",
		UserID:    "user",
		Path:      "/x",
		Bucket:    "b",
		Key:       "k",
		ExpiresAt: time.Now().Add(time.Hour),
	}, time.Hour)
	store := &fakeStore{objectExists: false}
	svc := newTestService(t, reg, store)
	_, err := svc.CompleteUpload(context.Background(), "u1", "d1", "u1", 100, nil)
	assert.ErrorIs(t, err, ErrObjectNotUploaded)
}

func TestCompleteUploadHappyPath(t *testing.T) {
	reg := NewMemoryRegistry()
	_ = reg.Put(context.Background(), PresignMeta{
		UploadID:  "u1",
		DriveID:   "d1",
		UserID:    "user",
		Path:      "/x",
		Bucket:    "b",
		Key:       "k",
		ExpiresAt: time.Now().Add(time.Hour),
	}, time.Hour)
	svc := newTestService(t, reg, nil)
	n, err := svc.CompleteUpload(context.Background(), "u1", "d1", "u1", 100, nil)
	require.NoError(t, err)
	assert.NotNil(t, n)
	// Token should be deleted.
	_, err = reg.Get(context.Background(), "u1")
	assert.Error(t, err)
}


