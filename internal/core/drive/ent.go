package drive

import (
	"context"
	"fmt"
	"time"

	"github.com/mandacode-labs/mdrive/ent"
	entdrive "github.com/mandacode-labs/mdrive/ent/drive"
	entdrivestorage "github.com/mandacode-labs/mdrive/ent/drivestorage"
	"github.com/mandacode-labs/mdrive/internal/crypto"
)

// entRepository implements domain.Repository using Ent.
type entRepository struct {
	client *ent.Client
	cipher crypto.Cipher
}

// NewRepository creates a new entRepository.
// cipher is optional; if nil, a crypto.NoOp cipher is used (not recommended for production).
func NewRepository(client *ent.Client, cipher crypto.Cipher) Repository {
	if cipher == nil {
		cipher = crypto.NoOp{}
	}
	return &entRepository{client: client, cipher: cipher}
}

// Create persists a drive and its storage config. It is tx-transparent:
// it uses whatever client the repository was constructed with. Callers
// that need this op to participate in a larger transaction must wrap it
// in WithTx at the service layer.
func (r *entRepository) Create(ctx context.Context, d *Drive, s *Storage) error {
	if _, err := r.client.Drive.Create().
		SetID(d.ID()).
		SetPublicID(d.PublicID()).
		SetName(d.Name()).
		SetNillableDescription(d.Description()).
		SetProvider(entdrive.Provider(d.Provider())).
		SetOwnerID(d.OwnerID()).
		SetNillableRootNodeID(d.RootNodeID()).
		Save(ctx); err != nil {
		return fmt.Errorf("create drive: %w", err)
	}

	secretKey, err := r.cipher.Encrypt([]byte(s.SecretKey()))
	if err != nil {
		return fmt.Errorf("encrypt secret key: %w", err)
	}

	if _, err := r.client.DriveStorage.Create().
		SetDriveID(s.DriveID()).
		SetBucket(s.Bucket()).
		SetNillableEndpoint(s.Endpoint()).
		SetRegion(s.Region()).
		SetAccessKey(s.AccessKey()).
		SetSecretKey(string(secretKey)).
		SetUsePathStyle(s.UsePathStyle()).
		Save(ctx); err != nil {
		return fmt.Errorf("create drive storage: %w", err)
	}

	return nil
}

func (r *entRepository) GetByID(ctx context.Context, id string) (*Drive, error) {
	d, err := r.client.Drive.Query().Where(entdrive.IDEQ(id)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return driveFromEnt(d), nil
}

func (r *entRepository) GetByPublicID(ctx context.Context, publicID string) (*Drive, error) {
	d, err := r.client.Drive.Query().Where(entdrive.PublicIDEQ(publicID)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return driveFromEnt(d), nil
}

func (r *entRepository) GetStorage(ctx context.Context, driveID string) (*Storage, error) {
	s, err := r.client.DriveStorage.Query().Where(entdrivestorage.DriveIDEQ(driveID)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	secretKey, err := r.cipher.Decrypt([]byte(s.SecretKey))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}
	return NewStorage(
		s.DriveID,
		s.Bucket,
		s.Endpoint,
		s.Region,
		s.AccessKey,
		string(secretKey),
		s.UsePathStyle,
	), nil
}

func (r *entRepository) Update(ctx context.Context, d *Drive) (*Drive, error) {
	updated, err := r.client.Drive.UpdateOneID(d.ID()).
		SetName(d.Name()).
		SetNillableDescription(d.Description()).
		SetNillableRootNodeID(d.RootNodeID()).
		SetNillableDeletedAt(d.DeletedAt()).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return driveFromEnt(updated), nil
}

func (r *entRepository) SoftDelete(ctx context.Context, id string) error {
	now := time.Now()
	_, err := r.client.Drive.UpdateOneID(id).
		SetDeletedAt(now).
		Save(ctx)
	return err
}

func (r *entRepository) Restore(ctx context.Context, id string) error {
	_, err := r.client.Drive.UpdateOneID(id).
		ClearDeletedAt().
		Save(ctx)
	return err
}

func (r *entRepository) Delete(ctx context.Context, id string) error {
	if _, err := r.client.DriveStorage.Delete().Where(entdrivestorage.DriveIDEQ(id)).Exec(ctx); err != nil {
		if !ent.IsNotFound(err) {
			return err
		}
	}
	return r.client.Drive.DeleteOneID(id).Exec(ctx)
}

func (r *entRepository) FindByOwner(ctx context.Context, ownerID string) ([]*Drive, error) {
	drives, err := r.client.Drive.Query().Where(entdrive.OwnerIDEQ(ownerID)).Where(entdrive.DeletedAtIsNil()).All(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*Drive, len(drives))
	for i, d := range drives {
		result[i] = driveFromEnt(d)
	}
	return result, nil
}

func (r *entRepository) FindDeleted(ctx context.Context, before time.Time, limit int) ([]*Drive, error) {
	drives, err := r.client.Drive.Query().
		Where(entdrive.DeletedAtNotNil()).
		Where(entdrive.DeletedAtLTE(before)).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*Drive, len(drives))
	for i, d := range drives {
		result[i] = driveFromEnt(d)
	}
	return result, nil
}

func (r *entRepository) FindDeletedByOwner(ctx context.Context, ownerID string) ([]*Drive, error) {
	drives, err := r.client.Drive.Query().
		Where(entdrive.OwnerIDEQ(ownerID)).
		Where(entdrive.DeletedAtNotNil()).
		All(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*Drive, len(drives))
	for i, d := range drives {
		result[i] = driveFromEnt(d)
	}
	return result, nil
}

// WithTx executes fn within a transaction.
func (r *entRepository) WithTx(ctx context.Context, fn func(Repository) error) error {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	txClient := tx.Client()
	txRepo := &entRepository{client: txClient, cipher: r.cipher}
	if err := fn(txRepo); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func driveFromEnt(e *ent.Drive) *Drive {
	if e == nil {
		return nil
	}
	return NewDrive(
		e.ID,
		e.PublicID,
		e.Name,
		e.Description,
		parseProvider(string(e.Provider)),
		e.OwnerID,
		e.RootNodeID,
		e.DeletedAt,
		e.CreateTime,
		e.UpdateTime,
	)
}

func parseProvider(s string) Provider {
	switch s {
	case "s3":
		return ProviderS3
	case "minio":
		return ProviderMinio
	default:
		return ProviderS3
	}
}
