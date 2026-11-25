package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/0xballo/fui/internal/domain"
)

// PlanRestoreDump describes replacing the archive with the content of a dump read from source
// (a file path or label); it touches nothing. The plan is always bulk.
func (s *Service) PlanRestoreDump(d ArchiveDump, source string) (RestorePlan, error) {
	if source == "" {
		return RestorePlan{}, newError(domain.ErrMissingField, map[string]any{"field": "source"}, "missing dump source")
	}
	if d.SchemaVersion <= 0 || len(d.Tables) == 0 {
		return RestorePlan{}, newError(domain.ErrInvalidValue, map[string]any{"source": source}, "the dump is empty or has no schema version")
	}
	counts := make(map[string]any, len(d.Tables))
	for _, t := range d.Tables {
		counts[t.Name] = len(t.Rows)
	}
	ch := Change{
		Op: OpUpdate, Entity: EntityArchive,
		Summary: fmt.Sprintf("replace the archive with the dump %q (%d tables)", source, len(d.Tables)),
		Fields:  counts,
	}
	return RestorePlan{Plan: Plan{Changes: []Change{ch}, Bulk: true}, Path: source}, nil
}

// ApplyRestoreDump replaces the archive with the dump. The store backs the current archive up
// and swaps atomically, so a failure leaves it untouched.
func (s *Service) ApplyRestoreDump(ctx context.Context, d ArchiveDump) error {
	ld, ok := s.backup.(ArchiveLoader)
	if !ok {
		return errors.New("restore dump: the archive adapter does not support loading a dump")
	}
	if err := ld.LoadArchive(ctx, d); err != nil {
		return fmt.Errorf("restore dump: %w", err)
	}
	return nil
}
