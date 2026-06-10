package inode

import (
	"context"
	"encoding/json"

	"github.com/mandacode-labs/mdrive/internal/core/inode/content"
	"github.com/mandacode-labs/mdrive/internal/errors"
)

func (s *Service) Link(ctx context.Context, dirID string, entry DirEntry) error {
	dir, err := s.GetByID(ctx, dirID)
	if err != nil {
		return err
	}
	if !dir.IsDir() {
		return errors.BadRequest("not a directory")
	}

	var c content.DirContent
	if dir.Content() != nil {
		if err := json.Unmarshal(dir.Content(), &c); err != nil {
			return errors.WrapInternal(err, "failed to parse directory content")
		}
	}

	for _, e := range c.Entries {
		if e.Name == entry.Name {
			return errors.Conflict("entry already exists: " + entry.Name)
		}
	}

	c.Entries = append(c.Entries, entry)
	raw, err := json.Marshal(c)
	if err != nil {
		return errors.WrapInternal(err, "failed to marshal directory content")
	}

	return s.Update(ctx, &UpdateCommand{
		ID:      dirID,
		Content: &raw,
	})
}
