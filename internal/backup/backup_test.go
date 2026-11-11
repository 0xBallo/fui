package backup_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/0xballo/fui/internal/backup"
)

func TestFileNameAndPath(t *testing.T) {
	ts := time.Date(2026, 11, 11, 23, 15, 30, 0, time.UTC)

	tests := []struct {
		reason  backup.Reason
		want    string
		wantErr bool
	}{
		{reason: backup.ReasonManual, want: "fui-20261111-231530-manual.db"},
		{reason: backup.ReasonImport, want: "fui-20261111-231530-import.db"},
		{reason: backup.ReasonMigrate, want: "fui-20261111-231530-migrate.db"},
		{reason: backup.ReasonBulk, want: "fui-20261111-231530-bulk.db"},
		{reason: backup.ReasonRestore, want: "fui-20261111-231530-restore.db"},
		{reason: "unsupported", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(string(tt.reason), func(t *testing.T) {
			got, err := backup.FileName(ts, tt.reason)
			if (err != nil) != tt.wantErr {
				t.Fatalf("FileName() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("FileName() = %q, want %q", got, tt.want)
			}

			path, err := backup.FilePath("/backups", ts, tt.reason)
			if (err != nil) != tt.wantErr {
				t.Fatalf("FilePath() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && path != filepath.Join("/backups", tt.want) {
				t.Errorf("FilePath() = %q, want %q", path, filepath.Join("/backups", tt.want))
			}
		})
	}
}

func TestParseName(t *testing.T) {
	tests := []struct {
		name       string
		wantReason backup.Reason
		wantYear   int
		wantErr    bool
	}{
		{name: "fui-20261111-231530-manual.db", wantReason: backup.ReasonManual, wantYear: 2026},
		{name: "fui-20260102-030405-import.db", wantReason: backup.ReasonImport, wantYear: 2026},
		{name: "fui-20251231-120000-migrate.db", wantReason: backup.ReasonMigrate, wantYear: 2025},
		{name: "other-20261111-231530-manual.db", wantErr: true},
		{name: "fui-20261111-manual.db", wantErr: true},
		{name: "fui-20261111-231530-unknown.db", wantErr: true},
		{name: "fui-20261111-231530-manual.txt", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, reason, err := backup.ParseName(tt.name)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseName(%q) error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}
			if !tt.wantErr {
				if reason != tt.wantReason {
					t.Errorf("reason = %q, want %q", reason, tt.wantReason)
				}
				if ts.Year() != tt.wantYear {
					t.Errorf("year = %d, want %d", ts.Year(), tt.wantYear)
				}
			}
		})
	}
}

func TestEnsureDir(t *testing.T) {
	tmp := t.TempDir()
	targetDir := filepath.Join(tmp, "sub", "backup_dir")

	if err := backup.EnsureDir(targetDir); err != nil {
		t.Fatalf("EnsureDir failed: %v", err)
	}

	info, err := os.Stat(targetDir)
	if err != nil {
		t.Fatalf("stat backup dir failed: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("expected dir perms 0700, got %#o", perm)
	}
}

func TestListAndRotate(t *testing.T) {
	dir := t.TempDir()

	// Empty directory
	files, err := backup.List(dir)
	if err != nil {
		t.Fatalf("List on empty dir returned error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}

	// Non-existent directory returns nil, nil
	missing, err := backup.List(filepath.Join(dir, "nonexistent"))
	if err != nil || missing != nil {
		t.Fatalf("expected nil, nil for missing dir, got %v, %v", missing, err)
	}

	// Create a sequence of backup files and some non-backup files
	baseTime := time.Date(2026, 11, 10, 10, 0, 0, 0, time.UTC)
	var createdNames []string
	for i := 0; i < 5; i++ {
		ts := baseTime.Add(time.Duration(i) * time.Hour)
		name, err := backup.FileName(ts, backup.ReasonManual)
		if err != nil {
			t.Fatalf("FileName failed: %v", err)
		}
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("backup data"), 0o600); err != nil {
			t.Fatalf("write backup file failed: %v", err)
		}
		createdNames = append(createdNames, name)
	}

	// Write unrelated files that should be ignored
	unrelated1 := filepath.Join(dir, "notes.txt")
	unrelated2 := filepath.Join(dir, "fui.db")
	if err := os.WriteFile(unrelated1, []byte("notes"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(unrelated2, []byte("db"), 0o600); err != nil {
		t.Fatal(err)
	}

	// List should return only the 5 backup files in chronological order
	listed, err := backup.List(dir)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(listed) != 5 {
		t.Fatalf("expected 5 files, got %d", len(listed))
	}
	for i, f := range listed {
		if f.Name != createdNames[i] {
			t.Errorf("file[%d] name = %q, want %q", i, f.Name, createdNames[i])
		}
	}

	// Rotate with keep=3 should delete the 2 oldest backup files
	deleted, err := backup.Rotate(dir, 3)
	if err != nil {
		t.Fatalf("Rotate failed: %v", err)
	}
	wantDeleted := []string{
		filepath.Join(dir, createdNames[0]),
		filepath.Join(dir, createdNames[1]),
	}
	if diff := cmp.Diff(wantDeleted, deleted); diff != "" {
		t.Errorf("unexpected deleted files (-want +got):\n%s", diff)
	}

	// Remaining backup files should be 3 (the newest)
	afterRotate, err := backup.List(dir)
	if err != nil {
		t.Fatalf("List after rotate failed: %v", err)
	}
	if len(afterRotate) != 3 {
		t.Fatalf("expected 3 files remaining, got %d", len(afterRotate))
	}
	wantRemaining := createdNames[2:]
	for i, f := range afterRotate {
		if f.Name != wantRemaining[i] {
			t.Errorf("remaining file[%d] = %q, want %q", i, f.Name, wantRemaining[i])
		}
	}

	// Non-backup files should still exist untouched
	if _, err := os.Stat(unrelated1); err != nil {
		t.Errorf("unrelated file 1 was deleted: %v", err)
	}
	if _, err := os.Stat(unrelated2); err != nil {
		t.Errorf("unrelated file 2 was deleted: %v", err)
	}

	// Rotating with keep >= count should do nothing
	noopDeleted, err := backup.Rotate(dir, 10)
	if err != nil || noopDeleted != nil {
		t.Errorf("expected nil, nil from no-op rotate, got %v, %v", noopDeleted, err)
	}

	// Rotate with negative keep should return error
	if _, err := backup.Rotate(dir, -1); err == nil {
		t.Error("expected error for negative keep, got nil")
	}

	// Rotate with keep=0 should delete all remaining matching files
	allDeleted, err := backup.Rotate(dir, 0)
	if err != nil {
		t.Fatalf("Rotate(0) failed: %v", err)
	}
	if len(allDeleted) != 3 {
		t.Errorf("expected 3 deleted files, got %d", len(allDeleted))
	}
	finalList, err := backup.List(dir)
	if err != nil || len(finalList) != 0 {
		t.Errorf("expected 0 files after keep=0, got %v, %v", finalList, err)
	}
}
