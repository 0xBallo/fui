// Package backup manages file-level database backup naming, listing, and rotation.
//
// Backup files follow the strict naming pattern fui-YYYYMMDD-HHMMSS-<reason>.db
// and are stored in a dedicated directory with 0700 permissions. Timestamps
// are always passed as parameters, ensuring deterministic testing without
// internal clock dependencies.
package backup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"
)

// Reason specifies the operational trigger that caused a backup.
type Reason string

const (
	// ReasonManual indicates a backup requested explicitly by the user.
	ReasonManual Reason = "manual"
	// ReasonImport indicates an automatic backup before a data import.
	ReasonImport Reason = "import"
	// ReasonMigrate indicates an automatic backup before running database migrations.
	ReasonMigrate Reason = "migrate"
	// ReasonBulk indicates an automatic backup before executing bulk modifications.
	ReasonBulk Reason = "bulk"
	// ReasonRestore indicates an automatic backup of current data before restoring another archive.
	ReasonRestore Reason = "restore"
)

// TimeFormat defines the layout YYYYMMDD-HHMMSS used in backup file names.
const TimeFormat = "20060102-150405"

var backupRegex = regexp.MustCompile(`^fui-(\d{8}-\d{6})-(manual|import|migrate|bulk|restore)\.db$`)

// IsValidReason reports whether r is one of the recognized backup reasons.
func IsValidReason(r Reason) bool {
	switch r {
	case ReasonManual, ReasonImport, ReasonMigrate, ReasonBulk, ReasonRestore:
		return true
	default:
		return false
	}
}

// File represents a single valid backup archive on disk.
type File struct {
	Path      string
	Name      string
	Time      time.Time
	Reason    Reason
	SizeBytes int64
	ModTime   time.Time
}

// FileName generates the canonical backup file name for a given timestamp and reason.
func FileName(t time.Time, reason Reason) (string, error) {
	if !IsValidReason(reason) {
		return "", fmt.Errorf("invalid backup reason %q", reason)
	}
	return fmt.Sprintf("fui-%s-%s.db", t.Format(TimeFormat), reason), nil
}

// FilePath returns the full canonical path for a backup file within dir.
func FilePath(dir string, t time.Time, reason Reason) (string, error) {
	name, err := FileName(t, reason)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}

// ParseName extracts the timestamp and reason from a backup file name.
func ParseName(name string) (time.Time, Reason, error) {
	matches := backupRegex.FindStringSubmatch(name)
	if matches == nil {
		return time.Time{}, "", fmt.Errorf("file %q does not match backup naming pattern", name)
	}

	parsedTime, err := time.Parse(TimeFormat, matches[1])
	if err != nil {
		return time.Time{}, "", fmt.Errorf("parse backup timestamp %q: %w", matches[1], err)
	}

	return parsedTime, Reason(matches[2]), nil
}

// EnsureDir creates the backup directory with 0700 permissions if it does not exist.
func EnsureDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create backup directory %q: %w", dir, err)
	}
	return nil
}

// List returns all backup files in dir matching the naming pattern,
// sorted chronologically from oldest to newest.
func List(dir string) ([]File, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read backup directory %q: %w", dir, err)
	}

	var files []File
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		t, reason, err := ParseName(entry.Name())
		if err != nil {
			// Skip files that do not match the backup naming pattern.
			continue
		}

		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("stat backup file %q: %w", entry.Name(), err)
		}

		files = append(files, File{
			Path:      filepath.Join(dir, entry.Name()),
			Name:      entry.Name(),
			Time:      t,
			Reason:    reason,
			SizeBytes: info.Size(),
			ModTime:   info.ModTime(),
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Time.Before(files[j].Time)
	})

	return files, nil
}

// Rotate removes the oldest backup files in dir matching the naming pattern,
// retaining at most the newest keep backups. It returns the paths of deleted files.
func Rotate(dir string, keep int) ([]string, error) {
	if keep < 0 {
		return nil, fmt.Errorf("rotate backups: keep must be non-negative, got %d", keep)
	}

	files, err := List(dir)
	if err != nil {
		return nil, fmt.Errorf("list backups for rotation: %w", err)
	}

	if len(files) <= keep {
		return nil, nil
	}

	numToDelete := len(files) - keep
	var deleted []string
	for i := 0; i < numToDelete; i++ {
		target := files[i].Path
		if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
			return deleted, fmt.Errorf("delete old backup %q: %w", target, err)
		}
		deleted = append(deleted, target)
	}

	return deleted, nil
}
