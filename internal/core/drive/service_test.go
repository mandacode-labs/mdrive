package drive

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

type fakeRepo struct{}

func (fakeRepo) Create(_ context.Context, _ *Drive, _ *Storage) error      { return nil }
func (fakeRepo) GetByID(_ context.Context, _ string) (*Drive, error)       { return nil, nil }
func (fakeRepo) GetByPublicID(_ context.Context, _ string) (*Drive, error) { return nil, nil }
func (fakeRepo) GetStorage(_ context.Context, _ string) (*Storage, error)  { return nil, nil }
func (fakeRepo) Update(_ context.Context, _ *Drive) (*Drive, error)        { return nil, nil }
func (fakeRepo) SoftDelete(_ context.Context, _ string) error              { return nil }
func (fakeRepo) Restore(_ context.Context, _ string) error                 { return nil }
func (fakeRepo) Delete(_ context.Context, _ string) error                  { return nil }
func (fakeRepo) FindByOwner(_ context.Context, _ string) ([]*Drive, error) { return nil, nil }
func (fakeRepo) FindDeleted(_ context.Context, _ time.Time, _ int) ([]*Drive, error) {
	return nil, nil
}
func (fakeRepo) FindDeletedByOwner(_ context.Context, _ string) ([]*Drive, error) {
	return nil, nil
}
func (fakeRepo) WithTx(_ context.Context, fn func(Repository) error) error { return fn(fakeRepo{}) }

type fakeExister struct{}

func (fakeExister) Exist(_ context.Context, _ string) (bool, error) { return true, nil }

type fakeRootDirectoryCreator struct{}

func (fakeRootDirectoryCreator) CreateRootDirectory(_ context.Context) (uuid.UUID, error) {
	return uuid.New(), nil
}

func TestWithTxRollsBackOnError(t *testing.T) {
	svc := &Service{
		repo:                 fakeRepo{},
		ownerChecker:          fakeExister{},
		rootDirectoryCreator: fakeRootDirectoryCreator{},
	}

	want := errors.New("boom")
	got := svc.WithTx(context.Background(), func(*Service) error { return want })
	assert.ErrorIs(t, got, want)
}
