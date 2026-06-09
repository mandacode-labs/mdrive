package fs

import (
	"context"
	"path"

	"golang.org/x/sync/errgroup"

	"github.com/mandacode-labs/retrowin-go/internal/core/dentry"
	"github.com/mandacode-labs/retrowin-go/internal/core/inode"
	"github.com/mandacode-labs/retrowin-go/internal/core/object"
	"github.com/mandacode-labs/retrowin-go/internal/errors"
	"github.com/mandacode-labs/retrowin-go/internal/logging"
)

const (
	// defaultDeleteWorkers is the maximum number of concurrent inode deletions.
	defaultDeleteWorkers = 20
	// maxWildcardMatches limits the number of files matched by a wildcard pattern.
	maxWildcardMatches = 10000
)

// deleteTarget represents a single item to be deleted.
type deleteTarget struct {
	path        string
	inodeID     string
	parentDirID string
	entryName   string
	isDir       bool
	isObject    bool
	objectID    string
}

// expandWildcards expands wildcard patterns in paths using path.Match.
func (s *service) expandWildcards(ctx context.Context, systemID string, paths []string) ([]string, error) {
	var result []string
	for _, p := range paths {
		// Check if path contains wildcard characters
		if !containsWildcard(p) {
			result = append(result, p)
			continue
		}

		// Split pattern into base directory and pattern
		dirPath := path.Dir(p)
		pattern := path.Base(p)

		// Resolve base directory
		dirInode, err := s.ResolvePath(ctx, systemID, dirPath)
		if err != nil {
			return nil, err
		}
		if !dirInode.IsDir() {
			return nil, errors.BadRequest("not a directory: " + dirPath)
		}

		entries, err := s.dentrySvc.ReadDir(ctx, dirInode.ID())
		if err != nil {
			return nil, err
		}

		matched := 0
		for _, entry := range entries {
			if entry.Name == "." || entry.Name == ".." {
				continue
			}
			isMatch, err := path.Match(pattern, entry.Name)
			if err != nil {
				return nil, errors.BadRequest("invalid wildcard pattern: " + pattern)
			}
			if isMatch {
				result = append(result, path.Join(dirPath, entry.Name))
				matched++
				if matched > maxWildcardMatches {
					return nil, errors.BadRequest("too many wildcard matches for pattern: " + p)
				}
			}
		}
	}
	return result, nil
}

func containsWildcard(p string) bool {
	for _, c := range p {
		if c == '*' || c == '?' || c == '[' {
			return true
		}
	}
	return false
}

// collectPath resolves a single path into delete targets.
// For directories with recursive=true, it traverses the entire tree.
func (s *service) collectPath(ctx context.Context, systemID string, p string, recursive bool) ([]*deleteTarget, error) {
	targetInode, err := s.ResolvePath(ctx, systemID, p)
	if err != nil {
		return nil, err
	}

	if !targetInode.IsDir() {
		// Single file or object
		parentDirPath := path.Dir(p)
		parentDir, err := s.ResolvePath(ctx, systemID, parentDirPath)
		if err != nil {
			return nil, err
		}

		t := &deleteTarget{
			path:        p,
			inodeID:     targetInode.ID(),
			parentDirID: parentDir.ID(),
			entryName:   path.Base(p),
			isDir:       false,
			isObject:    targetInode.IsObject(),
		}
		if targetInode.IsObject() {
			objID, _ := targetInode.ObjectID()
			t.objectID = objID
		}
		return []*deleteTarget{t}, nil
	}

	// It's a directory
	if !recursive {
		return nil, errors.BadRequest("directory not empty (use recursive=true): " + p)
	}

	// Recursive collection
	var targets []*deleteTarget

	// Collect all entries in the directory tree
	entries, err := s.collectDirectoryEntries(ctx, targetInode)
	if err != nil {
		return nil, err
	}

	// Resolve all child inodes in batch
	childIDs := make([]string, 0, len(entries))
	for _, e := range entries {
		childIDs = append(childIDs, e.InodeID)
	}

	var childInodes []*inode.Inode
	if len(childIDs) > 0 {
		childInodes, err = s.inodeSvc.Find(ctx, inode.Filter{IDs: childIDs})
		if err != nil {
			return nil, err
		}
	}

	// Build inode lookup map
	inodeMap := make(map[string]*inode.Inode, len(childInodes))
	for _, in := range childInodes {
		inodeMap[in.ID()] = in
	}

	// Create targets for each entry
	for _, e := range entries {
		childInode, ok := inodeMap[e.InodeID]
		if !ok {
			continue // Skip if inode not found
		}

		t := &deleteTarget{
			path:        e.Name,
			inodeID:     childInode.ID(),
			parentDirID: targetInode.ID(),
			entryName:   e.Name,
			isDir:       childInode.IsDir(),
			isObject:    childInode.IsObject(),
		}
		if childInode.IsObject() {
			objID, _ := childInode.ObjectID()
			t.objectID = objID
		}
		targets = append(targets, t)
	}

	// Add the directory itself as the last target
	dirParentPath := path.Dir(p)
	dirParent, err := s.ResolvePath(ctx, systemID, dirParentPath)
	if err != nil {
		return nil, err
	}

	targets = append(targets, &deleteTarget{
		path:        p,
		inodeID:     targetInode.ID(),
		parentDirID: dirParent.ID(),
		entryName:   path.Base(p),
		isDir:       true,
	})

	return targets, nil
}

// collectDirectoryEntries recursively collects all entries in a directory tree.
func (s *service) collectDirectoryEntries(ctx context.Context, dirInode *inode.Inode) ([]dentry.DirEntry, error) {
	entries, err := s.dentrySvc.ReadDir(ctx, dirInode.ID())
	if err != nil {
		return nil, err
	}

	var result []dentry.DirEntry
	for _, e := range entries {
		if e.Name == "." || e.Name == ".." {
			continue
		}
		result = append(result, e)

		childInode, err := s.inodeSvc.GetByID(ctx, e.InodeID)
		if err != nil {
			return nil, err
		}
		if childInode.IsDir() {
			childEntries, err := s.collectDirectoryEntries(ctx, childInode)
			if err != nil {
				return nil, err
			}
			result = append(result, childEntries...)
		}
	}

	return result, nil
}

// executeBatchDelete performs batch S3 delete, DB object delete, and parallel inode delete.
func (s *service) executeBatchDelete(ctx context.Context, targets []*deleteTarget) error {
	// Collect object IDs
	objectIDs := make([]string, 0)
	for _, t := range targets {
		if t.isObject && t.objectID != "" {
			objectIDs = append(objectIDs, t.objectID)
		}
	}

	// Fetch all objects to get storage keys for S3 batch delete
	bucketKeys := make(map[string][]string)
	if len(objectIDs) > 0 {
		objects, err := s.objectSvc.Find(ctx, object.ByIDs(objectIDs))
		if err != nil {
			logging.Ctx(ctx).Error().Err(err).Msg("failed to fetch objects for batch delete")
		} else {
			for _, obj := range objects {
				bucketKeys[obj.Bucket()] = append(bucketKeys[obj.Bucket()], obj.StorageKey())
			}
		}
	}

	// S3 batch delete
	for bucket, keys := range bucketKeys {
		if err := s.storage.DeleteObjects(ctx, bucket, keys); err != nil {
			logging.Ctx(ctx).Error().
				Str("bucket", bucket).
				Int("key_count", len(keys)).
				Err(err).
				Msg("failed to batch delete from S3")
		}
	}

	// Delete object records from DB
	if len(objectIDs) > 0 {
		if _, err := s.objectSvc.DeleteBatch(ctx, objectIDs); err != nil {
			logging.Ctx(ctx).Error().Err(err).Msg("failed to batch delete object records")
		}
	}

	// Parallel inode delete
	return s.parallelDeleteInodes(ctx, targets)
}

// parallelDeleteInodes deletes non-directory inodes in parallel.
func (s *service) parallelDeleteInodes(ctx context.Context, targets []*deleteTarget) error {
	// Collect non-directory inode IDs
	var inodeIDs []string
	for _, t := range targets {
		if !t.isDir {
			inodeIDs = append(inodeIDs, t.inodeID)
		}
	}

	if len(inodeIDs) == 0 {
		return nil
	}

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(defaultDeleteWorkers)

	for _, id := range inodeIDs {
		id := id // capture
		g.Go(func() error {
			if err := s.inodeSvc.Delete(ctx, id); err != nil {
				logging.Ctx(ctx).Warn().
					Str("inode_id", id).
					Err(err).
					Msg("failed to delete inode")
				return err
			}
			return nil
		})
	}

	return g.Wait()
}
