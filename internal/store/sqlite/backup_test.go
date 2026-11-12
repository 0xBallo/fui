package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/backup"
	"github.com/0xballo/fui/internal/clock"
	"github.com/0xballo/fui/internal/domain"
)

func TestBackupRotate(t *testing.T) {
	root := t.TempDir()
	store, clk := mustOpenStore(t, root, 2)

	for i := range 3 {
		clk.Set(time.Date(2026, 2, 1, 12, 0, i, 0, time.UTC))
		if _, err := store.Backup(context.Background(), app.BackupReasonManual); err != nil {
			t.Fatalf("backup %d: %v", i, err)
		}
	}

	files, err := backup.List(filepath.Join(root, "data", "backups"))
	if err != nil {
		t.Fatalf("backup.List: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("len(backups) = %d, want 2", len(files))
	}
}

func TestBackupSameSecondNameCollision(t *testing.T) {
	root := t.TempDir()
	store, _ := mustOpenStore(t, root, 2)

	gotNames := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		path, err := store.Backup(context.Background(), app.BackupReasonImport)
		if err != nil {
			t.Fatalf("backup %d: %v", i, err)
		}
		gotNames = append(gotNames, filepath.Base(path))
	}

	wantCreated := []string{
		"fui-20260102-030405-import.db",
		"fui-20260102-030405-import-2.db",
		"fui-20260102-030405-import-3.db",
	}
	if diff := cmp.Diff(wantCreated, gotNames); diff != "" {
		t.Fatalf("created names mismatch (-want +got):\n%s", diff)
	}

	files, err := backup.List(filepath.Join(root, "data", "backups"))
	if err != nil {
		t.Fatalf("backup.List: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("len(backups) = %d, want 2", len(files))
	}
	if files[0].Name != "fui-20260102-030405-import-2.db" || files[1].Name != "fui-20260102-030405-import-3.db" {
		t.Fatalf("remaining backups = [%s, %s], want [-2, -3]", files[0].Name, files[1].Name)
	}
}

func TestBackupRestoreRoundTrip(t *testing.T) {
	store, _ := newTestStore(t)

	original := insertAccountForTest(t, store, "Original")
	backupPath, err := store.Backup(context.Background(), app.BackupReasonManual)
	if err != nil {
		t.Fatalf("Backup: %v", err)
	}

	_ = insertAccountForTest(t, store, "Later")
	if err := store.Restore(context.Background(), backupPath); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	var names []string
	err = store.Read(context.Background(), func(r app.Repos) error {
		accounts, listErr := r.Accounts().List(context.Background())
		if listErr != nil {
			return listErr
		}
		for _, account := range accounts {
			names = append(names, account.Name)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(names) != 1 || names[0] != original.Name {
		t.Fatalf("accounts after restore = %v, want [%s]", names, original.Name)
	}
}

func TestRestoreRejectsCorruptedCandidate(t *testing.T) {
	store, _ := newTestStore(t)

	badPath := filepath.Join(t.TempDir(), "bad.db")
	if err := os.WriteFile(badPath, []byte("not a sqlite db"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	err := store.Restore(context.Background(), badPath)
	if !errors.Is(err, errDBCorrupted) {
		t.Fatalf("Restore error = %v, want db_corrupted", err)
	}
}

func TestRestoreLockedByOtherProcessKeepsCurrentDB(t *testing.T) {
	root := t.TempDir()
	store, _ := mustOpenStore(t, root, 5)

	_ = insertAccountForTest(t, store, "Original")
	candidate, err := store.Backup(context.Background(), app.BackupReasonManual)
	if err != nil {
		t.Fatalf("Backup: %v", err)
	}
	_ = insertAccountForTest(t, store, "Later")

	external, err := openDB(filepath.Join(root, "data", "fui.db"), 1, false)
	if err != nil {
		t.Fatalf("open external db: %v", err)
	}
	defer func() { _ = external.Close() }()

	readTx, err := external.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatalf("begin external read tx: %v", err)
	}
	defer func() { _ = readTx.Rollback() }()
	var n int
	if err := readTx.QueryRowContext(context.Background(), "SELECT count(*) FROM accounts").Scan(&n); err != nil {
		t.Fatalf("external read: %v", err)
	}

	err = store.Restore(context.Background(), candidate)
	if !errors.Is(err, errDBLocked) {
		t.Fatalf("Restore with external holder err = %v, want db_locked", err)
	}

	names := listAccountNames(t, store)
	sort.Strings(names)
	if len(names) != 2 || names[0] != "Later" || names[1] != "Original" {
		t.Fatalf("accounts after failed restore = %v, want [Later Original]", names)
	}
}

func listAccountNames(t *testing.T, store *Store) []string {
	t.Helper()

	var names []string
	err := store.Read(context.Background(), func(r app.Repos) error {
		accounts, listErr := r.Accounts().List(context.Background())
		if listErr != nil {
			return listErr
		}
		for _, account := range accounts {
			names = append(names, account.Name)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	return names
}

func mustOpenStore(t *testing.T, root string, keep int) (*Store, *clock.Fixed) {
	t.Helper()
	clk := clock.NewFixed(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	store, err := Open(context.Background(), Options{
		Path:       filepath.Join(root, "data", "fui.db"),
		BackupDir:  filepath.Join(root, "data", "backups"),
		BackupKeep: keep,
		Clock:      clk,
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store, clk
}

func insertAccountForTest(t *testing.T, store *Store, name string) domain.Account {
	t.Helper()
	eur, err := domain.ParseCurrency("EUR")
	if err != nil {
		t.Fatalf("ParseCurrency: %v", err)
	}
	opening, err := domain.NewMoney(0, eur)
	if err != nil {
		t.Fatalf("NewMoney: %v", err)
	}
	openedOn, err := domain.ParseDate("2026-01-01")
	if err != nil {
		t.Fatalf("ParseDate: %v", err)
	}

	var account domain.Account
	if err := store.WithTx(context.Background(), func(r app.Repos) error {
		var insertErr error
		account, insertErr = r.Accounts().Insert(context.Background(), domain.Account{
			Name:           name,
			Kind:           domain.AccountChecking,
			Currency:       eur,
			OpeningBalance: opening,
			OpenedOn:       openedOn,
		})
		return insertErr
	}); err != nil {
		t.Fatalf("WithTx: %v", err)
	}
	return account
}
