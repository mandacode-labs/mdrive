package dentry

import (
	"context"
	"encoding/json"

	"github.com/mandacode-labs/retrowin-go/internal/core/inode/content"
	"github.com/mandacode-labs/retrowin-go/internal/errors"
)

func (s *service) ReadDir(ctx context.Context, id string) ([]DirEntry, error) {
	in, err := s.inodeSvc.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !in.IsDir() {
		return nil, errors.BadRequest("not a directory")
	}

	if in.Content() == nil {
		return nil, nil
	}

	var c content.DirContent
	if err := json.Unmarshal(in.Content(), &c); err != nil {
		return nil, errors.WrapInternal(err, "failed to parse directory content")
	}

	return c.Entries, nil
}

func (s *service) Lookup(ctx context.Context, dirID string, name string) (*DirEntry, error) {
	entries, err := s.ReadDir(ctx, dirID)
	if err != nil {
		return nil, err
	}

	for i := range entries {
		if entries[i].Name == name {
			return &entries[i], nil
		}
	}

	return nil, errors.NotFound("entry not found: " + name)
}
