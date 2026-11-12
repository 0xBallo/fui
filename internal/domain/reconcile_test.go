package domain

import (
	"errors"
	"slices"
	"testing"

	"pgregory.net/rapid"
)

func TestReconcile(t *testing.T) {
	asOf := date(t, 2026, 11, 15)
	difference, unreconciled, err := Reconcile(
		money(t, 11_000, "EUR"),
		money(t, 10_000, "EUR"),
		[]Transaction{
			{ID: 11, Date: date(t, 2026, 11, 10), Amount: money(t, 900, "EUR"), Status: StatusBooked, Kind: KindPayment},
			{ID: 12, Date: date(t, 2026, 11, 12), Amount: money(t, 100, "EUR"), Status: StatusReconciled, Kind: KindPayment},
			{ID: 13, Date: date(t, 2026, 11, 16), Amount: money(t, -500, "EUR"), Status: StatusBooked, Kind: KindPayment},
			{ID: 14, Date: date(t, 2026, 11, 11), Amount: money(t, 100, "EUR"), Status: StatusPending, Kind: KindPayment},
		},
		asOf,
	)
	if err != nil {
		t.Fatal(err)
	}
	if difference.Minor() != 0 {
		t.Fatalf("difference = %d, want 0", difference.Minor())
	}
	if !slices.Equal(unreconciled, []TransactionID{11}) {
		t.Fatalf("unreconciled = %v, want [11]", unreconciled)
	}
}

func TestCloseReconciliation(t *testing.T) {
	asOf := date(t, 2026, 11, 15)
	got, err := CloseReconciliation(
		money(t, 10_950, "EUR"),
		money(t, 10_000, "EUR"),
		[]Transaction{
			{ID: 21, Date: date(t, 2026, 11, 10), Amount: money(t, 900, "EUR"), Status: StatusBooked, Kind: KindPayment},
			{ID: 22, Date: date(t, 2026, 11, 10), Amount: money(t, 50, "EUR"), Status: StatusReconciled, Kind: KindPayment},
		},
		asOf,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []TransactionID{21}) {
		t.Fatalf("close ids = %v, want [21]", got)
	}
	_, err = CloseReconciliation(
		money(t, 10_800, "EUR"),
		money(t, 10_000, "EUR"),
		[]Transaction{
			{ID: 21, Date: date(t, 2026, 11, 10), Amount: money(t, 900, "EUR"), Status: StatusBooked, Kind: KindPayment},
		},
		asOf,
	)
	if !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("non-zero difference err = %v", err)
	}
}

func TestReconcileErrors(t *testing.T) {
	asOf := date(t, 2026, 11, 15)
	_, _, err := Reconcile(Money{}, money(t, 0, "EUR"), nil, asOf)
	if !errors.Is(err, ErrMissingField) {
		t.Fatalf("missing statement balance err = %v", err)
	}
	_, _, err = Reconcile(money(t, 0, "EUR"), money(t, 0, "EUR"), nil, Date{})
	if !errors.Is(err, ErrMissingField) {
		t.Fatalf("missing date err = %v", err)
	}
	_, _, err = Reconcile(money(t, 0, "EUR"), money(t, 0, "USD"), nil, asOf)
	if !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("currency mismatch err = %v", err)
	}
}

func TestReconcileInsertionOrderProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		opening := money(t, rapid.Int64Range(-1<<40, 1<<40).Draw(t, "opening"), "EUR")
		asOf := genDate(t)
		count := rapid.IntRange(0, 30).Draw(t, "count")
		txs := make([]Transaction, 0, count)
		for i := range count {
			amount := rapid.Int64Range(-1<<20, 1<<20).Draw(t, "amount")
			d := genDate(t)
			if d.After(asOf) {
				d = asOf
			}
			status := StatusBooked
			if i%3 == 1 {
				status = StatusReconciled
			}
			if i%3 == 2 {
				status = StatusPending
			}
			txs = append(txs, Transaction{
				ID:        TransactionID(i + 1),
				AccountID: AccountID(i + 1),
				Date:      d,
				Amount:    money(t, amount, "EUR"),
				Status:    status,
				Kind:      KindPayment,
			})
		}
		internal, err := BalanceAt(opening, txs, asOf)
		if err != nil {
			t.Fatal(err)
		}
		pending, err := NewMoney(0, opening.Currency())
		if err != nil {
			t.Fatal(err)
		}
		for _, tx := range txs {
			if tx.Date.After(asOf) || tx.Status != StatusPending {
				continue
			}
			pending, err = pending.Add(tx.Amount)
			if err != nil {
				t.Fatal(err)
			}
		}
		statement, err := internal.Sub(pending)
		if err != nil {
			t.Fatal(err)
		}
		diffA, unreconciledA, err := Reconcile(statement, opening, txs, asOf)
		if err != nil {
			t.Fatal(err)
		}
		shuffled := slices.Clone(txs)
		slices.Reverse(shuffled)
		diffB, unreconciledB, err := Reconcile(statement, opening, shuffled, asOf)
		if err != nil {
			t.Fatal(err)
		}
		if diffA != diffB {
			t.Fatalf("difference changed with insertion order: %v vs %v", diffA, diffB)
		}
		if !slices.Equal(unreconciledA, unreconciledB) {
			t.Fatalf("unreconciled changed with insertion order: %v vs %v", unreconciledA, unreconciledB)
		}
	})
}
