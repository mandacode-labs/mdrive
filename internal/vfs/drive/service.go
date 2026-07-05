package drive

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/entx"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/logx"
)

// Service is the public contract of the drive domain: drive CRUD,
// soft-delete lifecycle, list queries, and storage lookup.
// Callers depend on this single interface; the unexported service
// struct is the only implementation.
//
// Permission checks are the caller's responsibility (the handler
// layer); this is the pure domain logic.
type Service interface {
	Create(ctx context.Context, actorID string, name, description string, cfg StorageConfig) (*Drive, uuid.UUID, error)
	GetByID(ctx context.Context, id string) (*Drive, error)
	GetByPublicID(ctx context.Context, publicID string) (*Drive, error)
	GetStorage(ctx context.Context, driveID string) (*Storage, error)
	Update(ctx context.Context, id string, name, description string) (*Drive, error)
	Delete(ctx context.Context, id string) error
	Restore(ctx context.Context, id string) (*Drive, error)
	Purge(ctx context.Context, id string) error
	ListByOwner(ctx context.Context, actorID string) ([]*Drive, error)
	ListDeletedByOwner(ctx context.Context, actorID string) ([]*Drive, error)
	ListDeletedForAdmin(ctx context.Context, isAdmin bool, before time.Time, limit int) ([]*Drive, error)
	WithTx(ctx context.Context, fn func(context.Context) error) error
}

// service is the only implementation of Service.
type service struct {
	repo                 Repository
	ownerChecker         OwnerChecker
	rootDirectoryCreator RootDirectoryCreator
	tm                   entx.TxManager
}

// NewService wires the drive domain.
func NewService(repo Repository, ownerChecker OwnerChecker, rootDirectoryCreator RootDirectoryCreator, tm entx.TxManager) Service {
	return &service{repo: repo, ownerChecker: ownerChecker, rootDirectoryCreator: rootDirectoryCreator, tm: tm}
}

var _ Service = (*service)(nil)

func (s *service) Create(ctx context.Context, actorID string, name, description string, cfg StorageConfig) (*Drive, uuid.UUID, error) {
	logx.Debug(ctx, "drive.svc.create.enter",
		slog.String("actor_id", actorID),
		slog.String("name", name),
	)
	if name == "" {
		return nil, uuid.Nil, errorx.New(errorx.KindInvalidArgument, "drive: invalid name (name is empty)")
	}
	if actorID == "" {
		return nil, uuid.Nil, errorx.New(errorx.KindInvalidArgument, "drive: owner_id is required")
	}
	if err := storageCfgValid(&cfg); err != nil {
		return nil, uuid.Nil, errorx.Wrap(err, "drive: storage invalid")
	}

	exists, err := s.ownerChecker.Exist(ctx, actorID)
	if err != nil {
		return nil, uuid.Nil, errorx.Wrap(err, fmt.Sprintf("drive: owner check failed (actor_id=%s)", actorID), errorx.KindUnavailable)
	}
	if !exists {
		return nil, uuid.Nil, errorx.New(errorx.KindPermissionDenied, "drive: owner not found (actor_id="+actorID+")")
	}
	logx.Debug(ctx, "drive.svc.create.owner_ok", slog.String("actor_id", actorID))

	id := ulid.Make().String()
	now := time.Now()
	var descriptionPtr *string
	if description != "" {
		descriptionPtr = &description
	}
	d := NewDrive(id, id, name, descriptionPtr, ProviderS3, actorID, nil, nil, now, now)
	storage := NewStorage(id, cfg.Bucket, cfg.Endpoint, cfg.Region,
		cfg.AccessKey, cfg.SecretKey, cfg.UsePathStyle)
	logx.Debug(ctx, "drive.svc.create.drive_built",
		slog.String("id", id),
		slog.Int("owner_id_len", len(actorID)),
	)

	rootID, err := s.rootDirectoryCreator.CreateRootDirectory(ctx)
	if err != nil {
		logx.Debug(ctx, "drive.svc.create.root_dir_failed",
			slog.String("id", id),
			slog.String("err", err.Error()),
		)
		return nil, uuid.Nil, errorx.Wrap(err, fmt.Sprintf("drive: root node creation failed (id=%s)", id), errorx.KindUnavailable)
	}
	d.SetRootNodeID(rootID)
	logx.Debug(ctx, "drive.svc.create.root_dir_ok",
		slog.String("id", id),
		slog.String("root_id", rootID.String()),
	)

	var updated *Drive
	err = s.tm.WithTx(ctx, func(ctx context.Context) error {
		logx.Debug(ctx, "drive.svc.create.tx.enter", slog.String("id", id))
		if err := s.repo.Create(ctx, d, storage); err != nil {
			logx.Debug(ctx, "drive.svc.create.tx.create_failed",
				slog.String("id", id),
				slog.String("err", err.Error()),
			)
			return errorx.Wrap(err, fmt.Sprintf("drive: repo create failed (id_len=%d, owner_id_len=%d, root_id=%s)", len(d.ID()), len(d.OwnerID()), rootID), errorx.KindUnavailable)
		}
		logx.Debug(ctx, "drive.svc.create.tx.create_ok", slog.String("id", id))
		u, err := s.repo.Update(ctx, d)
		if err != nil {
			logx.Debug(ctx, "drive.svc.create.tx.update_failed",
				slog.String("id", id),
				slog.String("err", err.Error()),
			)
			return errorx.Wrap(err, fmt.Sprintf("drive: repo update failed (id_len=%d)", len(d.ID())), errorx.KindUnavailable)
		}
		logx.Debug(ctx, "drive.svc.create.tx.update_ok",
			slog.String("id", id),
			slog.Bool("updated_nil", u == nil),
		)
		updated = u
		return nil
	})
	if err != nil {
		logx.Debug(ctx, "drive.svc.create.tx_failed",
			slog.String("id", id),
			slog.String("err", err.Error()),
		)
		return nil, uuid.Nil, errorx.Wrap(err, fmt.Sprintf("drive: tx failed (id=%s, root_id=%s)", id, rootID), errorx.KindUnavailable)
	}
	logx.Debug(ctx, "drive.svc.create.tx_committed",
		slog.String("id", id),
		slog.Bool("updated_nil", updated == nil),
	)
	return updated, rootID, nil
}

func (s *service) GetByID(ctx context.Context, id string) (*Drive, error) {
	d, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("drive: get by id (id=%s)", id))
	}
	if d == nil {
		return nil, errorx.New(errorx.KindNotFound, "drive: not found (id="+id+")")
	}
	return d, nil
}

func (s *service) GetByPublicID(ctx context.Context, publicID string) (*Drive, error) {
	d, err := s.repo.GetByPublicID(ctx, publicID)
	if err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("drive: get by public id (public_id=%s)", publicID))
	}
	if d == nil {
		return nil, errorx.New(errorx.KindNotFound, "drive: not found (public_id="+publicID+")")
	}
	return d, nil
}

func (s *service) GetStorage(ctx context.Context, driveID string) (*Storage, error) {
	st, err := s.repo.GetStorage(ctx, driveID)
	if err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("drive: get storage (drive_id=%s)", driveID))
	}
	if st == nil {
		return nil, errorx.New(errorx.KindNotFound, "drive: storage not found (drive_id="+driveID+")")
	}
	return st, nil
}

func (s *service) Update(ctx context.Context, id string, name, description string) (*Drive, error) {
	d, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("drive: update lookup (id=%s)", id))
	}
	if d == nil {
		return nil, errorx.New(errorx.KindNotFound, "drive: not found (id="+id+")")
	}
	newName := d.Name()
	if name != "" {
		newName = name
	}
	newDesc := d.Description()
	if description != "" {
		newDesc = &description
	}
	updated := NewDrive(
		d.ID(), d.PublicID(), newName, newDesc,
		d.Provider(), d.OwnerID(), d.RootNodeID(), d.DeletedAt(),
		d.CreatedAt(), time.Now(),
	)
	saved, err := s.repo.Update(ctx, updated)
	if err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("drive: update (id=%s)", id))
	}
	return saved, nil
}

func (s *service) Delete(ctx context.Context, id string) error {
	if _, err := s.GetByID(ctx, id); err != nil {
		return err
	}
	if err := s.repo.SoftDelete(ctx, id); err != nil {
		return errorx.Wrap(err, fmt.Sprintf("drive: soft delete (id=%s)", id))
	}
	return nil
}

func (s *service) Restore(ctx context.Context, id string) (*Drive, error) {
	d, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if d.DeletedAt() == nil {
		return nil, errorx.New(errorx.KindFailedPrecondition, "drive: not deleted (id="+id+")")
	}
	if err := s.repo.Restore(ctx, id); err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("drive: restore (id=%s)", id))
	}
	return s.GetByID(ctx, id)
}

func (s *service) Purge(ctx context.Context, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return errorx.Wrap(err, fmt.Sprintf("drive: purge (id=%s)", id))
	}
	return nil
}

func (s *service) ListDeletedForAdmin(ctx context.Context, isAdmin bool, before time.Time, limit int) ([]*Drive, error) {
	if !isAdmin {
		return nil, errorx.New(errorx.KindPermissionDenied, "permission: denied")
	}
	return s.repo.FindDeleted(ctx, before, limit)
}

func (s *service) ListDeletedByOwner(ctx context.Context, actorID string) ([]*Drive, error) {
	return s.repo.FindDeletedByOwner(ctx, actorID)
}

func (s *service) ListByOwner(ctx context.Context, actorID string) ([]*Drive, error) {
	return s.repo.FindByOwner(ctx, actorID)
}

func (s *service) WithTx(ctx context.Context, fn func(context.Context) error) error {
	return s.tm.WithTx(ctx, fn)
}

func storageCfgValid(cfg *StorageConfig) error {
	if cfg.Bucket == "" {
		return errorx.New(errorx.KindInvalidArgument, "drive: storage bucket is required")
	}
	if cfg.Region == "" {
		return errorx.New(errorx.KindInvalidArgument, "drive: storage region is required")
	}
	return nil
}
