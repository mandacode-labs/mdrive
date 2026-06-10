package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/mandacode-labs/mdrive/ent"
	entobject "github.com/mandacode-labs/mdrive/ent/object"
	domain "github.com/mandacode-labs/mdrive/internal/core/object"
)

// EntRepository implements domain.ObjectRepository using Ent.
type EntRepository struct {
	client *ent.Client
}

// NewRepository creates a new EntRepository.
func NewRepository(client *ent.Client) domain.ObjectRepository {
	return &EntRepository{client: client}
}

func (r *EntRepository) Create(ctx context.Context, params *domain.CreateParams) (*domain.Object, error) {
	builder := r.client.Object.Create().
		SetID(params.ID).
		SetProvider(entobject.Provider(string(params.Provider))).
		SetBucket(params.Bucket).
		SetSystemID(params.SystemID).
		SetStorageKey(params.StorageKey)

	if params.Status != "" {
		builder = builder.SetStatus(entobject.Status(string(params.Status)))
	}
	if params.Checksum != nil {
		builder = builder.SetChecksum(*params.Checksum)
	}
	if params.IdempotencyKey != nil {
		builder = builder.SetIdempotencyKey(*params.IdempotencyKey)
	}

	entObject, err := builder.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create object: %w", err)
	}

	return fromEnt(entObject), nil
}

func (r *EntRepository) GetByID(ctx context.Context, id string) (*domain.Object, error) {
	entObject, err := r.client.Object.Query().
		Where(entobject.ID(id)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get object: %w", err)
	}
	return fromEnt(entObject), nil
}

func (r *EntRepository) GetByStorageKey(ctx context.Context, systemID string, provider string, bucket string, storageKey string) (*domain.Object, error) {
	entObject, err := r.client.Object.Query().
		Where(
			entobject.SystemIDEQ(systemID),
			entobject.ProviderEQ(entobject.Provider(provider)),
			entobject.BucketEQ(bucket),
			entobject.StorageKeyEQ(storageKey),
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get object by storage key: %w", err)
	}
	return fromEnt(entObject), nil
}

func (r *EntRepository) GetByIdempotencyKey(ctx context.Context, systemID string, idempotencyKey string) (*domain.Object, error) {
	entObject, err := r.client.Object.Query().
		Where(
			entobject.SystemIDEQ(systemID),
			entobject.IdempotencyKeyEQ(idempotencyKey),
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get object by idempotency key: %w", err)
	}
	return fromEnt(entObject), nil
}

func (r *EntRepository) UpdateStatus(ctx context.Context, id string, status domain.Status) error {
	return r.client.Object.UpdateOneID(id).
		SetStatus(entobject.Status(string(status))).
		Exec(ctx)
}

func (r *EntRepository) Delete(ctx context.Context, id string) error {
	return r.client.Object.DeleteOneID(id).Exec(ctx)
}

func (r *EntRepository) DeleteBySystemID(ctx context.Context, systemID string) error {
	_, err := r.client.Object.Delete().Where(entobject.SystemIDEQ(systemID)).Exec(ctx)
	return err
}

func (r *EntRepository) Find(ctx context.Context, filter *domain.QueryFilter) ([]*domain.Object, error) {
	query := r.client.Object.Query()
	query = applyFilter(query, filter)

	entObjects, err := query.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find objects: %w", err)
	}
	return fromEntSlice(entObjects), nil
}

func (r *EntRepository) FindOne(ctx context.Context, filter *domain.QueryFilter) (*domain.Object, error) {
	query := r.client.Object.Query()
	query = applyFilter(query, filter)

	entObject, err := query.Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find object: %w", err)
	}
	return fromEnt(entObject), nil
}

func (r *EntRepository) FindPendingOlderThan(ctx context.Context, olderThan time.Duration) ([]*domain.Object, error) {
	threshold := time.Now().Add(-olderThan)

	entObjects, err := r.client.Object.Query().
		Where(
			entobject.StatusEQ(entobject.StatusPending),
			entobject.UpdateTimeLT(threshold),
		).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find pending objects: %w", err)
	}
	return fromEntSlice(entObjects), nil
}

func (r *EntRepository) FindActive(ctx context.Context) ([]*domain.Object, error) {
	entObjects, err := r.client.Object.Query().
		Where(entobject.StatusEQ(entobject.StatusActive)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find active objects: %w", err)
	}
	return fromEntSlice(entObjects), nil
}

func applyFilter(query *ent.ObjectQuery, filter *domain.QueryFilter) *ent.ObjectQuery {
	if filter == nil {
		return query
	}
	if filter.ID != nil {
		query = query.Where(entobject.ID(*filter.ID))
	}
	if len(filter.IDs) > 0 {
		query = query.Where(entobject.IDIn(filter.IDs...))
	}
	if filter.SystemID != nil {
		query = query.Where(entobject.SystemIDEQ(*filter.SystemID))
	}
	if filter.Provider != nil {
		query = query.Where(entobject.ProviderEQ(entobject.Provider(*filter.Provider)))
	}
	if filter.Bucket != nil {
		query = query.Where(entobject.BucketEQ(*filter.Bucket))
	}
	if filter.StorageKey != nil {
		query = query.Where(entobject.StorageKeyEQ(*filter.StorageKey))
	}
	if filter.Status != nil {
		query = query.Where(entobject.StatusEQ(entobject.Status(*filter.Status)))
	}
	if filter.IdempotencyKey != nil {
		query = query.Where(entobject.IdempotencyKeyEQ(*filter.IdempotencyKey))
	}
	return query
}

func fromEnt(e *ent.Object) *domain.Object {
	return domain.NewObject(
		e.ID,
		domain.Provider(string(e.Provider)),
		e.Bucket,
		e.SystemID,
		e.StorageKey,
		domain.Status(string(e.Status)),
		e.Checksum,
		e.IdempotencyKey,
		e.CreateTime,
		e.UpdateTime,
	)
}

func fromEntSlice(objects []*ent.Object) []*domain.Object {
	result := make([]*domain.Object, len(objects))
	for i, e := range objects {
		result[i] = fromEnt(e)
	}
	return result
}
