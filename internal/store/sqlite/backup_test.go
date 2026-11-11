package sqlite

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

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
