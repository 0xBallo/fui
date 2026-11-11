package sqlite

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

func TestWithTxRollbackOnError(t *testing.T) {
	store, _ := newTestStore(t)

	boom := errors.New("boom")
	err := store.WithTx(context.Background(), func(r app.Repos) error {
		_, insertErr := r.Categories().Insert(context.Background(), domain.Category{
			Name: "Food",
			Kind: domain.CategoryExpense,
		})
		if insertErr != nil {
			return insertErr
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("WithTx error = %v, want %v", err, boom)
	}

	err = store.Read(context.Background(), func(r app.Repos) error {
		categories, listErr := r.Categories().List(context.Background())
		if listErr != nil {
			return listErr
		}
		if len(categories) != 0 {
			t.Fatalf("categories after failed tx = %v, want empty", categories)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
}

func TestConcurrentWritersSerialize(t *testing.T) {
	store, _ := newTestStore(t)
	eur, _ := domain.ParseCurrency("EUR")
	opening, _ := domain.NewMoney(0, eur)
	openedOn, _ := domain.ParseDate("2026-01-01")

	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	errs := make(chan error, 2)

	write := func(name string) {
		defer wg.Done()
		<-start
		errs <- store.WithTx(context.Background(), func(r app.Repos) error {
			_, err := r.Accounts().Insert(context.Background(), domain.Account{
				Name:           name,
				Kind:           domain.AccountChecking,
				Currency:       eur,
				OpeningBalance: opening,
				OpenedOn:       openedOn,
			})
			return err
		})
	}

	go write("A")
	go write("B")
	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("writer failed: %v", err)
		}
	}

	err := store.Read(context.Background(), func(r app.Repos) error {
		accounts, listErr := r.Accounts().List(context.Background())
		if listErr != nil {
			return listErr
		}
		if len(accounts) != 2 {
			t.Fatalf("accounts = %d, want 2", len(accounts))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
}

func TestReadIsReadOnly(t *testing.T) {
	store, _ := newTestStore(t)

	err := store.Read(context.Background(), func(r app.Repos) error {
		_, insertErr := r.Categories().Insert(context.Background(), domain.Category{
			Name: "Food",
			Kind: domain.CategoryExpense,
		})
		return insertErr
	})
	if err == nil {
		t.Fatal("Read write should fail")
	}
}

func TestOpenSupportsSpecialCharactersInPath(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "db #1", "fui ?main%v1.db")
	store, err := Open(context.Background(), Options{
		Path:       path,
		BackupDir:  filepath.Join(root, "backups"),
		BackupKeep: 5,
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Size() == 0 {
		t.Fatalf("db file size is 0 for %q, expected migrated sqlite file", path)
	}
}

func TestConcurrentWithTxAcrossStoresReadThenWrite(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "data", "fui.db")
	backupDir := filepath.Join(root, "data", "backups")

	storeA, err := Open(context.Background(), Options{
		Path:       path,
		BackupDir:  backupDir,
		BackupKeep: 10,
	})
	if err != nil {
		t.Fatalf("Open storeA: %v", err)
	}
	t.Cleanup(func() { _ = storeA.Close() })

	storeB, err := Open(context.Background(), Options{
		Path:       path,
		BackupDir:  backupDir,
		BackupKeep: 10,
	})
	if err != nil {
		t.Fatalf("Open storeB: %v", err)
	}
	t.Cleanup(func() { _ = storeB.Close() })

	start := make(chan struct{})
	errs := make(chan error, 2)
	run := func(s *Store, name string) {
		<-start
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		errs <- s.WithTx(ctx, func(r app.Repos) error {
			if _, err := r.Categories().List(ctx); err != nil {
				return err
			}
			time.Sleep(120 * time.Millisecond)
			_, err := r.Categories().Insert(ctx, domain.Category{
				Name: name,
				Kind: domain.CategoryExpense,
			})
			return err
		})
	}

	go run(storeA, "c1")
	go run(storeB, "c2")
	close(start)

	for range 2 {
		if err := <-errs; err != nil {
			t.Fatalf("writer failed: %v", err)
		}
	}
}
