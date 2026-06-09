package inode

import (
	"context"

	"github.com/google/uuid"

	"github.com/mandacode-labs/retrowin-go/internal/core/inode/content"
	"github.com/mandacode-labs/retrowin-go/internal/errors"
)

// DirEntry represents a directory entry (filename to inode mapping).
type DirEntry = content.DirEntry

// Service implements filesystem inode operations.
// Methods on Service correspond to Linux's inode_operations and file_operations for directories.
type Service struct {
	repo InodeRepository
}

// NewService creates a new Service.
func NewService(repo InodeRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, cmd *CreateCommand) (*Inode, error) {
	inodeID := uuid.New().String()
	params := &CreateParams{
		ID:       inodeID,
		SystemID: cmd.SystemID,
		Mode:     cmd.Mode,
		UID:      cmd.UID,
		GID:      cmd.GID,
		Size:     cmd.Size,
		Flags:    cmd.Flags,
		Content:  cmd.Content,
	}
	return s.repo.Create(ctx, params)
}

func (s *Service) GetByID(ctx context.Context, id string) (*Inode, error) {
	inode, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if inode == nil {
		return nil, errors.NotFound("inode not found")
	}
	return inode, nil
}

func (s *Service) Update(ctx context.Context, cmd *UpdateCommand) error {
	return s.repo.Update(ctx, cmd)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *Service) DeleteBySystemID(ctx context.Context, systemID string) error {
	return s.repo.DeleteBySystemID(ctx, systemID)
}

func (s *Service) Find(ctx context.Context, filter Filter) ([]*Inode, error) {
	return s.repo.Find(ctx, &filter)
}

func (s *Service) FindOne(ctx context.Context, filter Filter) (*Inode, error) {
	inode, err := s.repo.FindOne(ctx, &filter)
	if err != nil {
		return nil, err
	}
	if inode == nil {
		return nil, errors.NotFound("inode not found")
	}
	return inode, nil
}

func (s *Service) UpdateLinkCount(ctx context.Context, id string, delta int) error {
	return s.repo.UpdateLinkCount(ctx, id, delta)
}
