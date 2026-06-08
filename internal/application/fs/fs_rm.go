package fs

import (
	"context"
	"path"

	"github.com/mandacode-labs/retrowin-go/internal/errors"
	"github.com/mandacode-labs/retrowin-go/internal/logging"
)

// Rm removes multiple paths, like Unix rm.
// Supports wildcards (*, ?, [seq]) in path names.
// If Recursive is true, directories and their contents are deleted recursively.
// All deletions are performed with batch S3 deletes and parallel DB operations.
func (s *service) Rm(ctx context.Context, cmd *RmCommand) (*RmResult, error) {
	if len(cmd.Paths) == 0 {
		return nil, errors.BadRequest("no paths provided")
	}

	result := &RmResult{}

	// Phase 1: Expand wildcards
	expandedPaths, err := s.expandWildcards(ctx, cmd.SystemID, cmd.Paths)
	if err != nil {
		return nil, err
	}

	// Phase 2: Collect targets (resolve paths + recursive traversal)
	// Collect per-path errors instead of failing the entire batch.
	var targets []*deleteTarget
	for _, p := range expandedPaths {
		t, err := s.collectPath(ctx, cmd.SystemID, p, cmd.Recursive)
		if err != nil {
			result.Errors = append(result.Errors, RmError{
				Path:  p,
				Error: err,
			})
			continue
		}
		targets = append(targets, t...)
	}

	if len(targets) == 0 {
		return result, nil
	}

	// Phase 3: Permission check
	var permErrors []RmError
	for _, t := range targets {
		targetInode, err := s.inodeSvc.GetByID(ctx, t.inodeID)
		if err != nil {
			permErrors = append(permErrors, RmError{
				Path:  t.path,
				Error: err,
			})
			continue
		}
		if err := s.checkPermFromContext(ctx, targetInode, AccessWrite); err != nil {
			permErrors = append(permErrors, RmError{
				Path:  t.path,
				Error: err,
			})
			continue
		}
	}

	// If any permission errors, return early
	if len(permErrors) > 0 {
		result.Errors = append(result.Errors, permErrors...)
		return result, nil
	}

	// Phase 4: Execute batch delete (S3 + DB + parallel inode delete for non-directories)
	if err := s.executeBatchDelete(ctx, targets); err != nil {
		logging.Ctx(ctx).Error().Err(err).Msg("batch delete failed")
		// Continue with unlink even if some deletions failed
	}

	// Phase 4b: Delete directory inodes (in order: children first, then parents)
	for _, t := range targets {
		if t.isDir {
			if err := s.inodeSvc.Delete(ctx, t.inodeID); err != nil {
				logging.Ctx(ctx).Warn().
					Str("inode_id", t.inodeID).
					Str("path", t.path).
					Err(err).
					Msg("failed to delete directory inode")
			}
		}
	}

	// Phase 5: Batch unlink entries from parent directories
	// Group targets by their parent directory for batch unlink
	parentEntries := make(map[string]map[string]struct{}) // dirID -> entryName -> exists
	for _, t := range targets {
		if t.parentDirID == "" || t.entryName == "" {
			continue
		}
		if parentEntries[t.parentDirID] == nil {
			parentEntries[t.parentDirID] = make(map[string]struct{})
		}
		parentEntries[t.parentDirID][t.entryName] = struct{}{}
	}

	// Perform batch unlink for each parent directory
	for dirID, entries := range parentEntries {
		names := make([]string, 0, len(entries))
		for name := range entries {
			names = append(names, name)
		}

		// Lock parent directory for concurrent safety
		unlock := s.dirLock.Lock(dirID)
		if err := s.dentrySvc.UnlinkBatch(ctx, dirID, names); err != nil {
			logging.Ctx(ctx).Error().
				Str("dir_id", dirID).
				Int("entry_count", len(names)).
				Err(err).
				Msg("batch unlink failed")
		}
		unlock()
	}

	// Phase 6: Assemble result
	// All successfully collected and processed targets are considered deleted.
	// Map unique paths from targets back to result.
	deletedSet := make(map[string]struct{})
	for _, t := range targets {
		deletedSet[path.Clean(t.path)] = struct{}{}
	}

	for _, p := range expandedPaths {
		cleanPath := path.Clean(p)
		if _, ok := deletedSet[cleanPath]; ok {
			result.Deleted = append(result.Deleted, p)
		}
	}

	return result, nil
}
