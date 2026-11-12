package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xballo/fui/internal/backup"
)

// Backup creates a full SQLite backup with VACUUM INTO and rotates old backups.
func (s *Store) Backup(ctx context.Context, reason string) (string, error) {
	r := backup.Reason(reason)
	if !backup.IsValidReason(r) {
		return "", fmt.Errorf("backup: invalid reason %q", reason)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.backupLocked(ctx, r)
}

func (s *Store) backupLocked(ctx context.Context, reason backup.Reason) (string, error) {
	if err := backup.EnsureDir(s.backupDir); err != nil {
		return "", fmt.Errorf("backup: ensure backup dir: %w", err)
	}
	path, err := backup.NextFilePath(s.backupDir, s.clock.Now().UTC(), reason)
	if err != nil {
		return "", fmt.Errorf("backup: resolve path: %w", err)
	}

	// #nosec G202 -- SQLite VACUUM INTO requires a literal path; quoteString escapes it.
	stmt := "VACUUM INTO " + quoteString(path)
	if _, err := s.writeDB.ExecContext(ctx, stmt); err != nil {
		return "", fmt.Errorf("backup: vacuum into: %w", mapSQLError(err))
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return "", fmt.Errorf("backup: chmod %q: %w", path, err)
	}
	if _, err := backup.Rotate(s.backupDir, s.backupKeep); err != nil {
		return "", fmt.Errorf("backup: rotate: %w", err)
	}
	return path, nil
}

// Restore validates the candidate backup, creates a restore backup of the current DB,
// atomically swaps files, and reopens the connections.
func (s *Store) Restore(ctx context.Context, path string) error {
	if path == "" {
		return fmt.Errorf("restore: missing path")
	}
	if err := validateRestoreCandidate(ctx, path); err != nil {
		return err
	}

	tempPath, err := s.copyCandidateToTemp(path)
	if err != nil {
		return err
	}
	defer func() {
		_ = os.Remove(tempPath)
	}()

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.backupLocked(ctx, backup.ReasonRestore); err != nil {
		return err
	}
	if _, err := s.writeDB.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return fmt.Errorf("restore: wal checkpoint: %w", mapSQLError(err))
	}

	oldWrite, oldRead := s.writeDB, s.readDB
	if err := closeDBs(oldRead, oldWrite); err != nil {
		return fmt.Errorf("restore: close active connections: %w", err)
	}
	if err := ensureWALSidecarsGone(s.path); err != nil {
		if reopenErr := s.reopenLocked(); reopenErr != nil {
			return fmt.Errorf("restore: keep current db after wal sidecar check: %w", errors.Join(err, reopenErr))
		}
		return err
	}

	if err := os.Rename(tempPath, s.path); err != nil {
		if reopenErr := s.reopenLocked(); reopenErr != nil {
			return fmt.Errorf("restore: replace db and reopen: %w", errors.Join(err, reopenErr))
		}
		return fmt.Errorf("restore: replace db file: %w", err)
	}

	if err := s.reopenLocked(); err != nil {
		return fmt.Errorf("restore: reopen database: %w", err)
	}
	return nil
}

func validateRestoreCandidate(ctx context.Context, path string) error {
	db, err := openDB(path, 1, false)
	if err != nil {
		return fmt.Errorf("restore: open candidate %q: %w", path, mapSQLError(err))
	}
	defer func() { _ = db.Close() }()

	var status string
	if err := db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&status); err != nil {
		return fmt.Errorf("restore: integrity check: %w", mapSQLError(err))
	}
	if status != "ok" {
		return &domainErrorf{
			sentinel: errDBCorrupted,
			format:   "restore: integrity check failed for %q: %s",
			args:     []any{path, status},
		}
	}

	userVersion, err := pragmaUserVersion(ctx, db)
	if err != nil {
		return fmt.Errorf("restore: user_version: %w", mapSQLError(err))
	}
	if userVersion > currentSchemaVersion {
		return &domainErrorf{
			sentinel: errDBNewerThanBinary,
			format:   "restore: candidate schema version %d is newer than binary version %d",
			args:     []any{userVersion, currentSchemaVersion},
		}
	}
	return nil
}

func (s *Store) copyCandidateToTemp(candidate string) (string, error) {
	file, err := os.CreateTemp(filepath.Dir(s.path), ".fui-restore-*.db")
	if err != nil {
		return "", fmt.Errorf("restore: create temp file: %w", err)
	}
	tempPath := file.Name()
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("restore: close temp file: %w", err)
	}

	// #nosec G304 -- restore candidate path is provided by the user command.
	src, err := os.Open(candidate)
	if err != nil {
		return "", fmt.Errorf("restore: open candidate %q: %w", candidate, err)
	}
	defer func() { _ = src.Close() }()

	// #nosec G304 -- tempPath is created in the DB directory by this process.
	dst, err := os.OpenFile(tempPath, os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", fmt.Errorf("restore: open temp file: %w", err)
	}
	defer func() { _ = dst.Close() }()

	if _, err := io.Copy(dst, src); err != nil {
		return "", fmt.Errorf("restore: copy candidate: %w", err)
	}
	if err := dst.Sync(); err != nil {
		return "", fmt.Errorf("restore: sync temp file: %w", err)
	}
	if err := os.Chmod(tempPath, 0o600); err != nil {
		return "", fmt.Errorf("restore: chmod temp file: %w", err)
	}
	return tempPath, nil
}

func (s *Store) reopenLocked() error {
	writeDB, err := openDB(s.path, 1, true)
	if err != nil {
		return err
	}
	readDB, err := openDB(s.path, defaultReadPoolSize, false)
	if err != nil {
		_ = writeDB.Close()
		return err
	}
	s.writeDB = writeDB
	s.readDB = readDB
	return nil
}

func closeDBs(dbs ...*sql.DB) error {
	var errs []error
	for _, db := range dbs {
		if db == nil {
			continue
		}
		errs = append(errs, db.Close())
	}
	return errors.Join(errs...)
}

func ensureWALSidecarsGone(path string) error {
	for _, suffix := range []string{"-wal", "-shm"} {
		sidecarPath := path + suffix
		if _, err := os.Stat(sidecarPath); err == nil {
			return &domainErrorf{
				sentinel: errDBLocked,
				format:   "restore: sidecar file %q still present after close",
				args:     []any{sidecarPath},
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("restore: stat sidecar %q: %w", sidecarPath, err)
		}
	}
	return nil
}

func quoteString(s string) string {
	return "'" + escapeSingleQuotes(s) + "'"
}

func escapeSingleQuotes(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
