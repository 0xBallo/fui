package app

import (
	"context"
	"fmt"

	"github.com/0xballo/fui/internal/domain"
)

// EntityArchive is the Change.Entity of a restore.
const EntityArchive = "archive"

// Backup writes a manual backup of the archive and returns its path.
func (s *Service) Backup(ctx context.Context) (string, error) {
	path, err := s.backup.Backup(ctx, BackupReasonManual)
	if err != nil {
		return "", fmt.Errorf("backup: %w", err)
	}
	return path, nil
}

// RestoreRequest names the backup file to restore.
type RestoreRequest struct {
	Path string
}

// RestorePlan is the (always bulk) plan to replace the archive with the backup at Path.
type RestorePlan struct {
	Plan
	Path string
}

// PlanRestore describes the restore; it touches nothing.
func (s *Service) PlanRestore(req RestoreRequest) (RestorePlan, error) {
	if req.Path == "" {
		return RestorePlan{}, newError(domain.ErrMissingField, map[string]any{"field": "path"}, "missing backup path")
	}
	ch := Change{
		Op: OpUpdate, Entity: EntityArchive,
		Summary: fmt.Sprintf("replace the archive with the backup %q", req.Path),
		Fields:  map[string]any{"path": req.Path},
	}
	return RestorePlan{Plan: Plan{Changes: []Change{ch}, Bulk: true}, Path: req.Path}, nil
}

// ApplyRestore restores the archive from the selected backup.
// The underlying store performs the pre-restore backup under its own lock.
func (s *Service) ApplyRestore(ctx context.Context, p RestorePlan) error {
	if err := s.backup.Restore(ctx, p.Path); err != nil {
		return fmt.Errorf("restore %q: %w", p.Path, err)
	}
	return nil
}
