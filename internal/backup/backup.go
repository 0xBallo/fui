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
	// ReasonReset indicates an automatic backup before the archive is reset. Rotate never deletes it.
	ReasonReset Reason = "reset"
)

// TimeFormat defines the layout YYYYMMDD-HHMMSS used in backup file names.
const TimeFormat = "20060102-150405"

var backupRegex = regexp.MustCompile(`^fui-(\d{8}-\d{6})-(manual|import|migrate|bulk|restore|reset)(?:-(\d+))?\.db$`)

// IsValidReason reports whether r is one of the recognized backup reasons.
func IsValidReason(r Reason) bool {
	switch r {
	case ReasonManual, ReasonImport, ReasonMigrate, ReasonBulk, ReasonRestore, ReasonReset:
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
	Counter   int
	SizeBytes int64
	ModTime   time.Time
}

// FileName generates the canonical backup file name for a given timestamp and reason.
func FileName(t time.Time, reason Reason) (string, error) {
	if !IsValidReason(reason) {
		return "", fmt.Errorf("invalid backup reason %q", reason)
	}
	return fileNameWithCounter(t, reason, 1), nil
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
	parsedTime, reason, _, err := parseNameDetails(name)
	if err != nil {
		return time.Time{}, "", err
	}
	return parsedTime, reason, nil
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
		t, reason, counter, err := parseNameDetails(entry.Name())
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
			Counter:   counter,
			SizeBytes: info.Size(),
			ModTime:   info.ModTime(),
		})
	}

	sort.Slice(files, func(i, j int) bool {
		if files[i].Time.Equal(files[j].Time) {
			if files[i].Counter != files[j].Counter {
				return files[i].Counter < files[j].Counter
			}
			return files[i].Name < files[j].Name
		}
		return files[i].Time.Before(files[j].Time)
	})

	return files, nil
}

// NextFilePath returns a collision-free backup path in dir.
// It keeps the canonical naming format and appends -2, -3, ... when needed.
func NextFilePath(dir string, t time.Time, reason Reason) (string, error) {
	name, err := FileName(t, reason)
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); err == nil {
		for counter := 2; ; counter++ {
			candidate := filepath.Join(dir, fileNameWithCounter(t, reason, counter))
			if _, statErr := os.Stat(candidate); errors.Is(statErr, os.ErrNotExist) {
				return candidate, nil
			} else if statErr != nil {
				return "", fmt.Errorf("stat backup file %q: %w", candidate, statErr)
			}
		}
	} else if errors.Is(err, os.ErrNotExist) {
		return path, nil
	} else {
		return "", fmt.Errorf("stat backup file %q: %w", path, err)
	}
}

func parseNameDetails(name string) (time.Time, Reason, int, error) {
	matches := backupRegex.FindStringSubmatch(name)
	if matches == nil {
		return time.Time{}, "", 0, fmt.Errorf("file %q does not match backup naming pattern", name)
	}

	parsedTime, err := time.Parse(TimeFormat, matches[1])
	if err != nil {
		return time.Time{}, "", 0, fmt.Errorf("parse backup timestamp %q: %w", matches[1], err)
	}

	counter := 1
	if matches[3] != "" {
		if _, scanErr := fmt.Sscanf(matches[3], "%d", &counter); scanErr != nil {
			return time.Time{}, "", 0, fmt.Errorf("parse backup counter %q: %w", matches[3], scanErr)
		}
		if counter < 2 {
			return time.Time{}, "", 0, fmt.Errorf("invalid backup counter %q", matches[3])
		}
	}

	return parsedTime, Reason(matches[2]), counter, nil
}

func fileNameWithCounter(t time.Time, reason Reason, counter int) string {
	if counter <= 1 {
		return fmt.Sprintf("fui-%s-%s.db", t.Format(TimeFormat), reason)
	}
	return fmt.Sprintf("fui-%s-%s-%d.db", t.Format(TimeFormat), reason, counter)
}

// Rotate removes the oldest backup files in dir matching the naming pattern,
// retaining at most the newest keep backups (reset backups are exempt). It returns the paths of deleted files.
func Rotate(dir string, keep int) ([]string, error) {
	if keep < 0 {
		return nil, fmt.Errorf("rotate backups: keep must be non-negative, got %d", keep)
	}

	all, err := List(dir)
	if err != nil {
		return nil, fmt.Errorf("list backups for rotation: %w", err)
	}
	// Reset backups stay until the user deletes them, so they neither count toward keep nor get removed.
	files := all[:0:0]
	for _, f := range all {
		if f.Reason != ReasonReset {
			files = append(files, f)
		}
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
