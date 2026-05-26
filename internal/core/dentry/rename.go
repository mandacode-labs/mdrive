package dentry

import (
	"context"
	"encoding/json"

	"github.com/mandacode-labs/retrowin-go/internal/core/inode"
	"github.com/mandacode-labs/retrowin-go/internal/core/inode/content"
	"github.com/mandacode-labs/retrowin-go/internal/errors"
)

func (s *service) RenameAt(ctx context.Context, dirID string, entry DirEntry) (string, error) {
	dir, err := s.inodeSvc.GetByID(ctx, dirID)
	if err != nil {
		return "", err
	}
	if !dir.IsDir() {
		return "", errors.BadRequest("not a directory")
	}

	var c content.DirContent
	if dir.Content() != nil {
		if err := json.Unmarshal(dir.Content(), &c); err != nil {
			return "", errors.WrapInternal(err, "failed to parse directory content")
		}
	}

	var prevInodeID string
	for i, e := range c.Entries {
		if e.Name == entry.Name {
			prevInodeID = e.InodeID
			c.Entries[i] = entry
			break
		}
	}
	if prevInodeID == "" {
		c.Entries = append(c.Entries, entry)
	}

	raw, err := json.Marshal(c)
	if err != nil {
		return "", errors.WrapInternal(err, "failed to marshal directory content")
	}

	if err := s.inodeSvc.Update(ctx, &inode.UpdateCommand{
		ID:      dirID,
		Content: &raw,
	}); err != nil {
		return "", err
	}

	return prevInodeID, nil
}
