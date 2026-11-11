package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

func TestConstraintMappings(t *testing.T) {
	store, _ := newTestStore(t)

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
	date, err := domain.ParseDate("2026-01-10")
	if err != nil {
		t.Fatalf("ParseDate: %v", err)
	}
	amount, err := domain.NewMoney(-100, eur)
	if err != nil {
		t.Fatalf("NewMoney: %v", err)
	}

	var account domain.Account
	var category domain.Category
	err = store.WithTx(context.Background(), func(r app.Repos) error {
		var txErr error
		account, txErr = r.Accounts().Insert(context.Background(), domain.Account{
			Name:           "Fineco",
			Kind:           domain.AccountChecking,
			Currency:       eur,
			OpeningBalance: opening,
			OpenedOn:       openedOn,
		})
		if txErr != nil {
			return txErr
		}
		category, txErr = r.Categories().Insert(context.Background(), domain.Category{
			Name: "Food",
			Kind: domain.CategoryExpense,
		})
		return txErr
	})
	if err != nil {
		t.Fatalf("seed data: %v", err)
	}

	err = store.WithTx(context.Background(), func(r app.Repos) error {
		_, txErr := r.Accounts().Insert(context.Background(), domain.Account{
			Name:           "fInEcO",
			Kind:           domain.AccountChecking,
			Currency:       eur,
			OpeningBalance: opening,
			OpenedOn:       openedOn,
		})
		return txErr
	})
	if !errors.Is(err, app.ErrAlreadyExists) {
		t.Fatalf("duplicate account name err = %v, want already_exists", err)
	}

	err = store.WithTx(context.Background(), func(r app.Repos) error {
		_, txErr := r.Transactions().Insert(context.Background(), domain.Transaction{
			AccountID:   account.ID,
			Date:        date,
			Amount:      amount,
			Description: "Coffee",
			Status:      domain.StatusBooked,
			Kind:        domain.KindPayment,
			CategoryID:  category.ID,
		})
		if txErr != nil {
			return txErr
		}
		return r.Categories().Delete(context.Background(), category.ID)
	})
	if !errors.Is(err, app.ErrInUse) {
		t.Fatalf("delete used category err = %v, want in_use", err)
	}

	err = store.WithTx(context.Background(), func(r app.Repos) error {
		_, txErr := r.Transactions().Insert(context.Background(), domain.Transaction{
			AccountID:   account.ID,
			Date:        date,
			Amount:      amount,
			Description: "One",
			Status:      domain.StatusBooked,
			Kind:        domain.KindPayment,
			ExternalID:  "ext-1",
		})
		if txErr != nil {
			return txErr
		}
		_, txErr = r.Transactions().Insert(context.Background(), domain.Transaction{
			AccountID:   account.ID,
			Date:        date,
			Amount:      amount,
			Description: "Two",
			Status:      domain.StatusBooked,
			Kind:        domain.KindPayment,
			ExternalID:  "ext-1",
		})
		return txErr
	})
	if !errors.Is(err, app.ErrAlreadyExists) {
		t.Fatalf("duplicate external id err = %v, want already_exists", err)
	}
}
