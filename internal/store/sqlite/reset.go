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

	tempPath, err := s.buildFresh(ctx, opts)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tempPath) }() // no-op once renamed

	if _, err := s.backupLocked(ctx, backup.ReasonReset); err != nil {
		return err
	}
	if _, err := s.writeDB.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return fmt.Errorf("reset: wal checkpoint: %w", mapSQLError(err))
	}
	if err := closeDBs(s.readDB, s.writeDB); err != nil {
		return fmt.Errorf("reset: close active connections: %w", err)
	}
	if err := ensureWALSidecarsGone(s.path); err != nil {
		return s.reopenAfter(err)
	}
	if err := os.Rename(tempPath, s.path); err != nil {
		return s.reopenAfter(fmt.Errorf("reset: replace db file: %w", err))
	}
	if err := s.reopenLocked(); err != nil {
		return fmt.Errorf("reset: reopen database: %w", err)
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
func (s *Store) buildFresh(ctx context.Context, opts app.ResetOptions) (string, error) {
	f, err := os.CreateTemp(filepath.Dir(s.path), ".fui-reset-*.db")
	if err != nil {
		return "", fmt.Errorf("reset: create temp file: %w", err)
	}
	tempPath := f.Name()
	// Open creates the file itself: a pre-existing one would count as an old archive to migrate and
	// trigger a needless "migrate" backup.
	if err := errors.Join(f.Close(), os.Remove(tempPath)); err != nil {
		return "", fmt.Errorf("reset: prepare temp file: %w", err)
	}
	if err := s.populateFresh(ctx, tempPath, opts); err != nil {
		_ = os.Remove(tempPath)
		return "", err
	}
	return tempPath, nil
}

func (s *Store) populateFresh(ctx context.Context, tempPath string, opts app.ResetOptions) (err error) {
	fresh, err := Open(ctx, Options{
		Path: tempPath, BackupDir: s.backupDir, BackupKeep: s.backupKeep, Clock: s.clock,
	})
	if err != nil {
		return fmt.Errorf("reset: open fresh archive: %w", err)
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
			return err
		}
		cats = kept.categories
	} else {
		cats = domain.DefaultCategories()
	}
	return fresh.WithTx(ctx, func(r app.Repos) error {
		for _, c := range cats {
			if _, err := r.Categories().Insert(ctx, domain.Category{Name: c.Name, Kind: c.Kind}); err != nil {
				return fmt.Errorf("reset: create category %q: %w", c.Name, err)
			}
		}
		for _, tag := range kept.tags {
			if _, err := r.Tags().Insert(ctx, domain.Tag{Name: tag.Name}); err != nil {
				return fmt.Errorf("reset: create tag %q: %w", tag.Name, err)
			}
		}
		for _, p := range kept.payees {
			if _, err := r.Payees().Insert(ctx, domain.Payee{Name: p.Name, Aliases: p.Aliases}); err != nil {
				return fmt.Errorf("reset: create payee %q: %w", p.Name, err)
			}
		}
		for _, rule := range kept.rules {
			if _, err := r.Rules().Insert(ctx, rule); err != nil {
				return fmt.Errorf("reset: create rule %q: %w", rule.Name, err)
			}
		}
		return nil
	})
}

// keptSettings is what a --keep-settings reset carries over.
type keptSettings struct {
	categories []domain.Category
	tags       []domain.Tag
	payees     []domain.Payee
	rules      []domain.Rule
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
	return nil
}
