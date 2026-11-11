package sqlite

import (
	"context"
	"errors"
	"sync"
	"testing"

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
