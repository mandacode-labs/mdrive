package drive

import (
	"context"
	"fmt"

	"github.com/mandacode-labs/mdrive/ent"
	entdrive "github.com/mandacode-labs/mdrive/ent/drive"
	entdrivestorage "github.com/mandacode-labs/mdrive/ent/drivestorage"
	"github.com/mandacode-labs/mdrive/internal/crypto"
)

// EntRepository implements domain.Repository using Ent.
type EntRepository struct {
	client *ent.Client
	cipher crypto.Cipher
}

// NewRepository creates a new EntRepository.
// cipher is optional; if nil, a crypto.NoOp cipher is used (not recommended for production).
func NewRepository(client *ent.Client, cipher crypto.Cipher) Repository {
	if cipher == nil {
		cipher = crypto.NoOp{}
	}
	return &EntRepository{client: client, cipher: cipher}
}

func (r *EntRepository) Create(ctx context.Context, d *Drive, s *Storage) error {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}

	if _, err := tx.Drive.Create().
		SetID(d.ID()).
		SetPublicID(d.PublicID()).
		SetName(d.Name()).
		SetNillableDescription(d.Description()).
		SetProvider(entdrive.Provider(d.Provider())).
		SetOwnerID(d.OwnerID()).
		SetNillableRootNodeID(d.RootNodeID()).
		Save(ctx); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("create drive: %w", err)
	}

	secretKey, err := r.cipher.Encrypt([]byte(s.SecretKey()))
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("encrypt secret key: %w", err)
	}

	if _, err := tx.DriveStorage.Create().
		SetDriveID(s.DriveID()).
		SetBucket(s.Bucket()).
		SetNillableEndpoint(s.Endpoint()).
		SetRegion(s.Region()).
		SetAccessKey(s.AccessKey()).
		SetSecretKey(string(secretKey)).
		SetUsePathStyle(s.UsePathStyle()).
		Save(ctx); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("create drive storage: %w", err)
	}

	return tx.Commit()
}

func (r *EntRepository) GetByID(ctx context.Context, id string) (*Drive, error) {
	d, err := r.client.Drive.Query().Where(entdrive.IDEQ(id)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return driveFromEnt(d), nil
}

func (r *EntRepository) GetByPublicID(ctx context.Context, publicID string) (*Drive, error) {
	d, err := r.client.Drive.Query().Where(entdrive.PublicIDEQ(publicID)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return driveFromEnt(d), nil
}

func (r *EntRepository) GetStorage(ctx context.Context, driveID string) (*Storage, error) {
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

func (r *EntRepository) Update(ctx context.Context, d *Drive) (*Drive, error) {
	updated, err := r.client.Drive.UpdateOneID(d.ID()).
		SetName(d.Name()).
		SetNillableDescription(d.Description()).
		SetNillableRootNodeID(d.RootNodeID()).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return driveFromEnt(updated), nil
}

func (r *EntRepository) Delete(ctx context.Context, id string) error {
	if _, err := r.client.DriveStorage.Delete().Where(entdrivestorage.DriveIDEQ(id)).Exec(ctx); err != nil {
		if !ent.IsNotFound(err) {
			return err
		}
	}
	return r.client.Drive.DeleteOneID(id).Exec(ctx)
}

func (r *EntRepository) FindByOwner(ctx context.Context, ownerID string) ([]*Drive, error) {
	drives, err := r.client.Drive.Query().Where(entdrive.OwnerIDEQ(ownerID)).All(ctx)
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
func (r *EntRepository) WithTx(ctx context.Context, fn func(Repository) error) error {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	txClient := tx.Client()
	txRepo := &EntRepository{client: txClient, cipher: r.cipher}
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
