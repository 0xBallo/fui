package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/0xballo/fui/internal/backup"
	"github.com/0xballo/fui/internal/clock"
)

func TestOpenMigratesEmptyDatabase(t *testing.T) {
	store, _ := newMigrateTestStore(t)

	version, err := pragmaUserVersion(context.Background(), store.writeDB)
	if err != nil {
		t.Fatalf("pragmaUserVersion: %v", err)
	}
	if version != currentSchemaVersion {
		t.Fatalf("user_version = %d, want %d", version, currentSchemaVersion)
	}
}

func TestSchemaGolden(t *testing.T) {
	store, _ := newMigrateTestStore(t)

	got, err := collectSchemaDump(context.Background(), store.writeDB)
	if err != nil {
		t.Fatalf("collectSchemaDump: %v", err)
	}
	wantBytes, err := os.ReadFile(filepath.Join("testdata", "schema.golden"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	want := string(wantBytes)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("schema dump mismatch (-want +got):\n%s", diff)
	}
}

func TestOpenRejectsNewerDatabase(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, "data", "fui.db")
	backupDir := filepath.Join(root, "data", "backups")
	clk := clock.NewFixed(time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC))

	db := mustOpenRawDB(t, dbPath)
	t.Cleanup(func() { _ = db.Close() })
	mustExec(t, db, "PRAGMA user_version = 999")
	mustClose(t, db)

	_, err := Open(context.Background(), Options{
		Path:       dbPath,
		BackupDir:  backupDir,
		BackupKeep: 10,
		Clock:      clk,
	})
	if !errors.Is(err, errDBNewerThanBinary) {
		t.Fatalf("Open error = %v, want db_newer_than_binary", err)
	}
}

func TestOpenCreatesMigrateBackupForExistingDatabase(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, "data", "fui.db")
	backupDir := filepath.Join(root, "data", "backups")
	clk := clock.NewFixed(time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC))

	db := mustOpenRawDB(t, dbPath)
	mustExec(t, db, "CREATE TABLE legacy (id INTEGER PRIMARY KEY, name TEXT)")
	mustExec(t, db, "INSERT INTO legacy (name) VALUES ('x')")
	mustClose(t, db)

	store, err := Open(context.Background(), Options{
		Path:       dbPath,
		BackupDir:  backupDir,
		BackupKeep: 10,
		Clock:      clk,
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	files, err := backup.List(backupDir)
	if err != nil {
		t.Fatalf("backup.List: %v", err)
	}
	if len(files) != 1 || files[0].Reason != backup.ReasonMigrate {
		t.Fatalf("backups = %+v, want one migrate backup", files)
	}
}

func TestOpenMapsMigrationFailure(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, "data", "fui.db")
	backupDir := filepath.Join(root, "data", "backups")
	clk := clock.NewFixed(time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC))

	db := mustOpenRawDB(t, dbPath)
	mustExec(t, db, "CREATE TABLE accounts (id INTEGER PRIMARY KEY)")
	mustClose(t, db)

	_, err := Open(context.Background(), Options{
		Path:       dbPath,
		BackupDir:  backupDir,
		BackupKeep: 10,
		Clock:      clk,
	})
	if !errors.Is(err, errMigrationFailed) {
		t.Fatalf("Open error = %v, want migration_failed", err)
	}
}

func mustOpenRawDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	db, err := openDB(path, 1, false)
	if err != nil {
		t.Fatalf("openDB: %v", err)
	}
	return db
}

func mustExec(t *testing.T, db *sql.DB, stmt string) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), stmt); err != nil {
		t.Fatalf("exec %q: %v", stmt, err)
	}
}

func mustClose(t *testing.T, db *sql.DB) {
	t.Helper()
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}
}

func newMigrateTestStore(t *testing.T) (*Store, *clock.Fixed) {
	t.Helper()

	baseTime := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	clk := clock.NewFixed(baseTime)

	root := t.TempDir()
	store, err := Open(context.Background(), Options{
		Path:         filepath.Join(root, "data", "fui.db"),
		BackupDir:    filepath.Join(root, "data", "backups"),
		BackupKeep:   10,
		ReadPoolSize: 4,
		Clock:        clk,
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := store.Close(); closeErr != nil {
			t.Fatalf("Close: %v", closeErr)
		}
	})
	return store, clk
}
