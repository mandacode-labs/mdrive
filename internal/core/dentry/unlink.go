package dentry

import (
	"context"
	"encoding/json"

	"github.com/mandacode-labs/retrowin-go/internal/core/inode"
	"github.com/mandacode-labs/retrowin-go/internal/core/inode/content"
	"github.com/mandacode-labs/retrowin-go/internal/errors"
)

func (s *service) Unlink(ctx context.Context, dirID string, name string) error {
	dir, err := s.inodeSvc.GetByID(ctx, dirID)
	if err != nil {
		return err
	}
	if !dir.IsDir() {
		return errors.BadRequest("not a directory")
	}

	var c content.DirContent
	if dir.Content() == nil {
		return nil
	}
	if err := json.Unmarshal(dir.Content(), &c); err != nil {
		return errors.WrapInternal(err, "failed to parse directory content")
	}

	filtered := make([]content.DirEntry, 0, len(c.Entries))
	found := false
	for _, e := range c.Entries {
		if e.Name == name {
			found = true
			continue
		}
		filtered = append(filtered, e)
	}
	if !found {
		return nil
	}

	c.Entries = filtered
	raw, err := json.Marshal(c)
	if err != nil {
		return errors.WrapInternal(err, "failed to marshal directory content")
	}

	return s.inodeSvc.Update(ctx, &inode.UpdateCommand{
		ID:      dirID,
		Content: &raw,
	})
}
