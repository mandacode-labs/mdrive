package object

import (
	"context"
	"time"
)

// ObjectRepository defines the interface for object data access.
type ObjectRepository interface {
	Create(ctx context.Context, params *CreateParams) (*Object, error)
	GetByID(ctx context.Context, id string) (*Object, error)
	GetByStorageKey(ctx context.Context, systemID string, provider string, bucket string, storageKey string) (*Object, error)
	GetByIdempotencyKey(ctx context.Context, systemID string, idempotencyKey string) (*Object, error)
	UpdateStatus(ctx context.Context, id string, status Status) error
	Delete(ctx context.Context, id string) error
	DeleteBySystemID(ctx context.Context, systemID string) error
	Find(ctx context.Context, filter *QueryFilter) ([]*Object, error)
	FindOne(ctx context.Context, filter *QueryFilter) (*Object, error)
	FindPendingOlderThan(ctx context.Context, olderThan time.Duration) ([]*Object, error)
	FindActive(ctx context.Context) ([]*Object, error)
}

// CreateParams for creating a new object (repository layer).
type CreateParams struct {
	ID             string
	Provider       Provider
	Bucket         string
	SystemID       string
	StorageKey     string
	Status         Status
	Checksum       *string
	IdempotencyKey *string
}

// QueryFilter for querying objects (repository layer).
type QueryFilter struct {
	ID             *string
	IDs            []string
	SystemID       *string
	Provider       *string
	Bucket         *string
	StorageKey     *string
	Status         *string
	IdempotencyKey *string
}
