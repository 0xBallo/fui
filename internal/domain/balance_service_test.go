package domain

import (
	"errors"
	"slices"
	"testing"

	"pgregory.net/rapid"
)

func TestBalanceAt(t *testing.T) {
	opening := money(t, 10_000, "EUR")
	upTo := date(t, 2026, 11, 15)
	movements := []Transaction{
		{Date: date(t, 2026, 11, 10), Amount: money(t, -1_000, "EUR")},
		{Date: date(t, 2026, 11, 15), Amount: money(t, 250, "EUR")},
		{Date: date(t, 2026, 11, 16), Amount: money(t, -999, "EUR")},
	}
	got, err := BalanceAt(opening, movements, upTo)
	if err != nil {
		t.Fatalf("BalanceAt: %v", err)
	}
	if got.Minor() != 9_250 {
		t.Fatalf("minor = %d, want 9250", got.Minor())
	}
}

func TestBalanceAtErrors(t *testing.T) {
	if _, err := BalanceAt(Money{}, nil, date(t, 2026, 1, 1)); !errors.Is(err, ErrMissingField) {
		t.Fatalf("zero currency err = %v", err)
	}
	if _, err := BalanceAt(money(t, 0, "EUR"), nil, Date{}); !errors.Is(err, ErrMissingField) {
		t.Fatalf("zero date err = %v", err)
	}
	if _, err := BalanceAt(money(t, 0, "EUR"), []Transaction{{Amount: money(t, 1, "EUR")}}, date(t, 2026, 1, 1)); !errors.Is(err, ErrMissingField) {
		t.Fatalf("movement zero date err = %v", err)
	}
	if _, err := BalanceAt(money(t, 0, "EUR"), []Transaction{{Date: date(t, 2026, 1, 1), Amount: money(t, 1, "USD")}}, date(t, 2026, 1, 1)); !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("currency mismatch err = %v", err)
	}
}

func TestBalanceAtInsertionOrderProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		opening := money(t, rapid.Int64Range(-1<<40, 1<<40).Draw(t, "opening"), "EUR")
		upTo := genDate(t)
		count := rapid.IntRange(0, 20).Draw(t, "count")
		movements := make([]Transaction, 0, count)
		for i := 0; i < count; i++ {
			delta := rapid.Int64Range(-1<<20, 1<<20).Draw(t, "amount")
			d := genDate(t)
			if d.After(upTo) {
				d = upTo
			}
			movements = append(movements, Transaction{
				AccountID: AccountID(i + 1),
				Date:      d,
				Amount:    money(t, delta, "EUR"),
				Status:    StatusBooked,
				Kind:      KindPayment,
			})
		}
		gotA, err := BalanceAt(opening, movements, upTo)
		if err != nil {
			t.Fatal(err)
		}
		shuffled := slices.Clone(movements)
		slices.Reverse(shuffled)
		gotB, err := BalanceAt(opening, shuffled, upTo)
		if err != nil {
			t.Fatal(err)
		}
		if gotA != gotB {
			t.Fatalf("order changed balance: %v vs %v", gotA, gotB)
		}
	})
}
