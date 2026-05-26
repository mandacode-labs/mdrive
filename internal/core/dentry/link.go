package dentry

import (
	"context"
	"encoding/json"

	"github.com/mandacode-labs/retrowin-go/internal/core/inode"
	"github.com/mandacode-labs/retrowin-go/internal/core/inode/content"
	"github.com/mandacode-labs/retrowin-go/internal/errors"
)

func (s *service) Link(ctx context.Context, dirID string, entry DirEntry) error {
	dir, err := s.inodeSvc.GetByID(ctx, dirID)
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

	return s.inodeSvc.Update(ctx, &inode.UpdateCommand{
		ID:      dirID,
		Content: &raw,
	})
}
