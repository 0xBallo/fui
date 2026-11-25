package sqlite

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/backup"
	"github.com/0xballo/fui/internal/domain"
)

var _ app.Resetter = (*Store)(nil)

// Reset builds a fresh archive in a temp file next to the current one (migrations plus the default
// categories, or with KeepSettings the current categories), takes a "reset" backup of the current
// archive, then atomically renames the fresh file over it and reopens. Every step before the
// rename leaves the current archive untouched.
func (s *Store) Reset(ctx context.Context, opts app.ResetOptions) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tempPath, warnings, err := s.buildFresh(ctx, opts)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tempPath) }() // no-op once renamed

	if err := s.swapLocked(ctx, tempPath, backup.ReasonReset, "reset"); err != nil {
		return err
	}
	if opts.OnWarning != nil {
		for _, w := range warnings {
			opts.OnWarning(w)
		}
	}
	return nil
}

// swapLocked backs up the current archive with the given reason, then atomically renames the
// prepared file over it and reopens. Every step before the rename leaves the current archive
// untouched. The caller holds s.mu and removes tempPath on failure.
func (s *Store) swapLocked(ctx context.Context, tempPath string, reason backup.Reason, op string) error {
	if _, err := s.backupLocked(ctx, reason); err != nil {
		return err
	}
	if _, err := s.writeDB.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return fmt.Errorf("%s: wal checkpoint: %w", op, mapSQLError(err))
	}
	if err := closeDBs(s.readDB, s.writeDB); err != nil {
		return fmt.Errorf("%s: close active connections: %w", op, err)
	}
	if err := ensureWALSidecarsGone(s.path); err != nil {
		return s.reopenAfter(err)
	}
	if err := os.Rename(tempPath, s.path); err != nil {
		return s.reopenAfter(fmt.Errorf("%s: replace db file: %w", op, err))
	}
	if err := s.reopenLocked(); err != nil {
		return fmt.Errorf("%s: reopen database: %w", op, err)
	}
	return nil
}

// reopenAfter reopens the current archive after a failed swap so the store stays usable.
func (s *Store) reopenAfter(cause error) error {
	if err := s.reopenLocked(); err != nil {
		return errors.Join(cause, err)
	}
	return cause
}

// buildFresh creates the new archive as a closed, sidecar-free temp file in the archive directory.
func (s *Store) buildFresh(ctx context.Context, opts app.ResetOptions) (string, []string, error) {
	f, err := os.CreateTemp(filepath.Dir(s.path), ".fui-reset-*.db")
	if err != nil {
		return "", nil, fmt.Errorf("reset: create temp file: %w", err)
	}
	tempPath := f.Name()
	// Open creates the file itself: a pre-existing one would count as an old archive to migrate and
	// trigger a needless "migrate" backup.
	if err := errors.Join(f.Close(), os.Remove(tempPath)); err != nil {
		return "", nil, fmt.Errorf("reset: prepare temp file: %w", err)
	}
	warnings, err := s.populateFresh(ctx, tempPath, opts)
	if err != nil {
		_ = os.Remove(tempPath)
		return "", nil, err
	}
	return tempPath, warnings, nil
}

func (s *Store) populateFresh(ctx context.Context, tempPath string, opts app.ResetOptions) (warnings []string, err error) {
	fresh, err := Open(ctx, Options{
		Path: tempPath, BackupDir: s.backupDir, BackupKeep: s.backupKeep, Clock: s.clock,
	})
	if err != nil {
		return nil, fmt.Errorf("reset: open fresh archive: %w", err)
	}
	defer func() {
		if cerr := fresh.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("reset: close fresh archive: %w", cerr)
		}
	}()

	var kept keptSettings
	var cats []domain.Category
	if opts.KeepSettings {
		if err := s.readSettings(ctx, &kept); err != nil {
			return nil, err
		}
		cats = kept.categories
	} else {
		cats = domain.DefaultCategories()
	}
	remap := newIDRemap()
	err = fresh.WithTx(ctx, func(r app.Repos) error {
		if err := insertCategories(ctx, r.Categories(), cats, remap); err != nil {
			return err
		}
		if err := insertTags(ctx, r.Tags(), kept.tags, remap); err != nil {
			return err
		}
		if err := insertPayees(ctx, r.Payees(), kept.payees, remap); err != nil {
			return err
		}
		if warnings, err = insertRules(ctx, r.Rules(), kept.rules, remap); err != nil {
			return err
		}
		return insertSavedReports(ctx, r.SavedReports(), kept.savedReports)
	})
	if err != nil {
		return nil, err
	}
	return warnings, fresh.insertPendingRefs(ctx, kept.pendingRefs)
}

func insertCategories(ctx context.Context, repo app.CategoryRepo, cats []domain.Category, remap idRemap) error {
	for _, c := range cats {
		n, err := repo.Insert(ctx, domain.Category{Name: c.Name, Kind: c.Kind})
		if err != nil {
			return fmt.Errorf("reset: create category %q: %w", c.Name, err)
		}
		remap.categories[c.ID] = n.ID
	}
	return nil
}

func insertTags(ctx context.Context, repo app.TagRepo, tags []domain.Tag, remap idRemap) error {
	for _, tag := range tags {
		n, err := repo.Insert(ctx, domain.Tag{Name: tag.Name})
		if err != nil {
			return fmt.Errorf("reset: create tag %q: %w", tag.Name, err)
		}
		remap.tags[tag.ID] = n.ID
	}
	return nil
}

func insertPayees(ctx context.Context, repo app.PayeeRepo, payees []domain.Payee, remap idRemap) error {
	for _, p := range payees {
		n, err := repo.Insert(ctx, domain.Payee{Name: p.Name, Aliases: p.Aliases})
		if err != nil {
			return fmt.Errorf("reset: create payee %q: %w", p.Name, err)
		}
		remap.payees[p.ID] = n.ID
	}
	return nil
}

func insertRules(ctx context.Context, repo app.RuleRepo, rules []domain.Rule, remap idRemap) ([]string, error) {
	var warnings []string
	for _, rule := range rules {
		mapped, warns, ok := remap.remapRule(rule)
		warnings = append(warnings, warns...)
		if !ok {
			continue
		}
		if _, err := repo.Insert(ctx, mapped); err != nil {
			return nil, fmt.Errorf("reset: create rule %q: %w", rule.Name, err)
		}
	}
	return warnings, nil
}

func insertSavedReports(ctx context.Context, repo app.SavedReportRepo, reports []app.SavedReport) error {
	for _, report := range reports {
		if _, err := repo.Insert(ctx, report); err != nil {
			return fmt.Errorf("reset: create saved report %q: %w", report.Name, err)
		}
	}
	return nil
}

// keptSettings is what a --keep-settings reset carries over.
type keptSettings struct {
	categories   []domain.Category
	tags         []domain.Tag
	payees       []domain.Payee
	rules        []domain.Rule
	savedReports []app.SavedReport
	pendingRefs  []pendingRef
}

// readSettings lists the settings without taking s.mu (Reset already holds it).
func (s *Store) readSettings(ctx context.Context, out *keptSettings) error {
	tx, err := s.readDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("reset: begin read: %w", mapSQLError(err))
	}
	defer func() { _ = tx.Rollback() }()
	r := newRepos(tx, true)
	if out.categories, err = r.Categories().List(ctx); err != nil {
		return fmt.Errorf("reset: list categories: %w", err)
	}
	if out.tags, err = r.Tags().List(ctx); err != nil {
		return fmt.Errorf("reset: list tags: %w", err)
	}
	if out.payees, err = r.Payees().List(ctx); err != nil {
		return fmt.Errorf("reset: list payees: %w", err)
	}
	if out.rules, err = r.Rules().List(ctx); err != nil {
		return fmt.Errorf("reset: list rules: %w", err)
	}
	if out.savedReports, err = r.SavedReports().List(ctx); err != nil {
		return fmt.Errorf("reset: list saved reports: %w", err)
	}
	if out.pendingRefs, err = s.readPendingRefs(ctx); err != nil {
		return err
	}
	return nil
}
