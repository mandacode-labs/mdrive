package vfs

import (
	"context"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/fs/s3"
)

// resolvePresigner returns the S3 presigner for a given
// superblock. Per-superblock storage config takes priority;
// missing config falls back to the default IRSA presigner.
func (v *vfs) resolvePresigner(ctx context.Context, sbID uuid.UUID) (s3.Presigner, error) {
	storage, err := v.storageOp.GetBySuperblock(ctx, sbID)
	if err != nil {
		return nil, err
	}
	if storage == nil {
		// No per-drive config — use default IRSA
		return v.presigner, nil
	}
	return s3.NewPresigner(storage)
}