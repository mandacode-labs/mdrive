package fs

import (
	"context"
	"encoding/json"

	"github.com/mandacode-labs/retrowin-go/ent"
	"github.com/mandacode-labs/retrowin-go/internal/core/dentry"
	"github.com/mandacode-labs/retrowin-go/internal/core/inode"
	"github.com/mandacode-labs/retrowin-go/internal/core/inode/content"
	inoderepo "github.com/mandacode-labs/retrowin-go/internal/core/inode/repository"
	"github.com/mandacode-labs/retrowin-go/internal/core/object"
	objectrepo "github.com/mandacode-labs/retrowin-go/internal/core/object/repository"
	"github.com/mandacode-labs/retrowin-go/internal/errors"
)

// AtomicUpload completes an upload and links the file to the filesystem atomically.
// Uses a single transaction to ensure:
//  1. Object is marked active
//  2. Inode is created
//  3. Parent directory is locked and updated
//  4. Old inode is unlinked (if replacing)
//
// On failure, all changes are rolled back. The object remains pending and can be retried.
func (s *service) AtomicUpload(ctx context.Context, cmd *AtomicUploadCommand) (*inode.Inode, error) {
	if cmd.ObjectID == "" {
		return nil, errors.BadRequest("object_id is required")
	}
	if cmd.SystemID == "" {
		return nil, errors.BadRequest("system_id is required")
	}

	var resultInode *inode.Inode

	err := s.withTx(ctx, func(tx *ent.Tx) error {
		// Create tx-scoped services
		objSvc, inodeSvc, dentrySvc := s.txServices(tx)

		// 1. Mark object as active (verifies storage existence)
		obj, err := objSvc.CompleteUpload(ctx, cmd.ObjectID)
		if err != nil {
			return err
		}

		// 2. Get object size from storage
		size, err := objSvc.GetObjectSize(ctx, obj.ID())
		if err != nil {
			return err
		}

		// 3. Create inode with object reference
		c := &content.ObjectContent{ObjectID: obj.ID()}
		cBytes, err := json.Marshal(c)
		if err != nil {
			return errors.WrapInternal(err, "failed to marshal object content")
		}

		mode := cmd.Mode
		if mode == 0 {
			mode = inode.ModeObject | inode.PermOwnerRW | inode.PermGroupRX | inode.PermOtherR
		}

		uid, gid, err := s.resolveIDs(ctx, cmd.SystemID, 0, 0)
		if err != nil {
			return err
		}

		newInode, err := inodeSvc.Create(ctx, &inode.CreateCommand{
			SystemID: cmd.SystemID,
			Mode:     mode,
			UID:      uid,
			GID:      gid,
			Size:     size,
			Flags:    cmd.Flags,
			Content:  cBytes,
		})
		if err != nil {
			return errors.WrapInternal(err, "failed to create inode")
		}

		// 4. Resolve parent directory
		parentDir, err := s.resolvePathTx(ctx, inodeSvc, dentrySvc, cmd.SystemID, cmd.DirPath)
		if err != nil {
			return err
		}

		// 5. Lock parent directory (serialization for concurrent directory operations)
		unlock := s.dirLock.Lock(parentDir.ID())
		defer unlock()

		// 6. Atomically replace or add directory entry
		entry := dentry.DirEntry{
			Name:     cmd.FileName,
			InodeID:  newInode.ID(),
			FileType: uint8(inode.ModeObject >> 12),
		}
		prevInodeID, err := dentrySvc.RenameAt(ctx, parentDir.ID(), entry)
		if err != nil {
			return err
		}

		// 7. Clean up previous inode if replaced (best-effort, ignore not found)
		if prevInodeID != "" {
			_ = s.deleteInodeTx(ctx, inodeSvc, objSvc, prevInodeID)
		}

		resultInode = newInode
		return nil
	})

	if err != nil {
		return nil, err
	}
	return resultInode, nil
}

// withTx executes the given function within a transaction.
func (s *service) withTx(ctx context.Context, fn func(*ent.Tx) error) error {
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return errors.WrapInternal(err, "failed to begin transaction")
	}

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return errors.WrapInternal(rbErr, "transaction rollback failed")
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return errors.WrapInternal(err, "failed to commit transaction")
	}
	return nil
}

// txServices creates transaction-scoped service instances.
func (s *service) txServices(tx *ent.Tx) (object.ObjectService, inode.InodeService, dentry.DentryService) {
	objRepo := objectrepo.NewRepository(tx.Client())
	objSvc := object.NewService(objRepo, s.storage)

	inodeRepo := inoderepo.NewRepository(tx.Client())
	inodeSvc := inode.NewService(inodeRepo)

	dentrySvc := dentry.NewService(inodeSvc)

	return objSvc, inodeSvc, dentrySvc
}

// resolvePathTx resolves a path using transaction-scoped services.
func (s *service) resolvePathTx(ctx context.Context, inodeSvc inode.InodeService, dentrySvc dentry.DentryService, systemID string, pathStr string) (*inode.Inode, error) {
	// Use a temporary fs service with tx-scoped dependencies for path resolution
	txFs := &service{
		inodeSvc:  inodeSvc,
		dentrySvc: dentrySvc,
		userSvc:   s.userSvc,
	}
	return txFs.ResolvePath(ctx, systemID, pathStr)
}

// deleteInodeTx deletes an inode and its associated object within a transaction.
func (s *service) deleteInodeTx(ctx context.Context, inodeSvc inode.InodeService, objSvc object.ObjectService, id string) error {
	in, err := inodeSvc.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if in.IsObject() {
		objectID, err := in.ObjectID()
		if err == nil && objectID != "" {
			_ = objSvc.Delete(ctx, objectID)
		}
	}

	return inodeSvc.Delete(ctx, id)
}
