package inode

import (
	"context"
	"encoding/json"

	"github.com/mandacode-labs/retrowin-go/internal/core/inode/content"
	"github.com/mandacode-labs/retrowin-go/internal/errors"
)

func (s *service) Unlink(ctx context.Context, dirID string, name string) error {
	return s.UnlinkBatch(ctx, dirID, []string{name})
}

func (s *service) UnlinkBatch(ctx context.Context, dirID string, names []string) error {
	if len(names) == 0 {
		return nil
	}

	dir, err := s.GetByID(ctx, dirID)
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

	nameSet := make(map[string]struct{}, len(names))
	for _, name := range names {
		nameSet[name] = struct{}{}
	}

	filtered := make([]content.DirEntry, 0, len(c.Entries))
	for _, e := range c.Entries {
		if _, ok := nameSet[e.Name]; ok {
			continue
		}
		filtered = append(filtered, e)
	}

	c.Entries = filtered
	raw, err := json.Marshal(c)
	if err != nil {
		return errors.WrapInternal(err, "failed to marshal directory content")
	}

	return s.Update(ctx, &UpdateCommand{
		ID:      dirID,
		Content: &raw,
	})
}
