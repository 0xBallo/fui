package domain

import (
	"errors"
	"slices"
	"testing"

	"pgregory.net/rapid"
)

func tickedFixture(t *testing.T) (Money, []Transaction, Date) {
	t.Helper()
	txs := []Transaction{
		{ID: 1, Date: date(t, 2026, 11, 10), Amount: money(t, 900, "EUR"), Status: StatusBooked, Kind: KindPayment},
		{ID: 2, Date: date(t, 2026, 11, 12), Amount: money(t, 100, "EUR"), Status: StatusReconciled, Kind: KindPayment},
		{ID: 3, Date: date(t, 2026, 11, 16), Amount: money(t, -500, "EUR"), Status: StatusBooked, Kind: KindPayment},
		{ID: 4, Date: date(t, 2026, 11, 11), Amount: money(t, 70, "EUR"), Status: StatusPending, Kind: KindPayment},
		{ID: 5, Date: date(t, 2026, 11, 11), Amount: money(t, -30, "EUR"), Status: StatusBooked, Kind: KindPayment},
	}
	return money(t, 10_000, "EUR"), txs, date(t, 2026, 11, 15)
}

func TestReconcileTicked(t *testing.T) {
	opening, txs, asOf := tickedFixture(t)
	tests := []struct {
		name      string
		ticked    []TransactionID
		wantDiff  int64
		wantUntck []TransactionID
	}{
		{"none ticked", nil, 11_000 - 10_100, []TransactionID{1, 5}},
		{"partial", []TransactionID{1}, 0, []TransactionID{5}},
		{"all", []TransactionID{1, 5}, 11_000 - 10_970, nil},
		{"ignores pending later and unknown", []TransactionID{1, 3, 4, 99}, 0, []TransactionID{5}},
		{"duplicates count once", []TransactionID{1, 1, 5, 5}, 11_000 - 10_970, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			diff, unticked, err := ReconcileTicked(money(t, 11_000, "EUR"), opening, txs, tc.ticked, asOf)
			if err != nil {
				t.Fatal(err)
			}
			if diff.Minor() != tc.wantDiff {
				t.Fatalf("difference = %d, want %d", diff.Minor(), tc.wantDiff)
			}
			if !slices.Equal(unticked, tc.wantUntck) {
				t.Fatalf("unticked = %v, want %v", unticked, tc.wantUntck)
			}
		})
	}
}

func TestReconcileTickedErrors(t *testing.T) {
	opening, txs, asOf := tickedFixture(t)
	eur := money(t, 1, "EUR")
	usd := money(t, 1, "USD")
	badDate := []Transaction{{ID: 1, Amount: eur, Status: StatusBooked}}
	tests := []struct {
		name       string
		statement  Money
		opening    Money
		txs        []Transaction
		asOf       Date
		wantTarget error
	}{
		{"missing statement", Money{}, opening, txs, asOf, ErrMissingField},
		{"missing date", eur, opening, txs, Date{}, ErrMissingField},
		{"missing opening", eur, Money{}, txs, asOf, ErrMissingField},
		{"tx without date", eur, opening, badDate, asOf, ErrMissingField},
		{"currency mismatch", usd, opening, txs, asOf, ErrCurrencyMismatch},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := ReconcileTicked(tc.statement, tc.opening, tc.txs, nil, tc.asOf)
			if !errors.Is(err, tc.wantTarget) {
				t.Fatalf("err = %v, want %v", err, tc.wantTarget)
			}
		})
	}
	mixed := []Transaction{{ID: 1, Date: asOf, Amount: usd, Status: StatusReconciled}}
	if _, _, err := ReconcileTicked(eur, opening, mixed, nil, asOf); !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("mixed currency err = %v", err)
	}
}

func TestReconcileTickedOrderAndDuplicatesProperty(t *testing.T) {
	opening, txs, asOf := tickedFixture(t)
	statement := money(t, 11_000, "EUR")
	rapid.Check(t, func(rt *rapid.T) {
		ids := rapid.SliceOf(rapid.SampledFrom([]TransactionID{1, 2, 3, 4, 5, 99})).Draw(rt, "ticked")
		want, wantU, err := ReconcileTicked(statement, opening, txs, ids, asOf)
		if err != nil {
			rt.Fatal(err)
		}
		shuffled := rapid.Permutation(ids).Draw(rt, "shuffled")
		dups := append(slices.Clone(shuffled), ids...)
		got, gotU, err := ReconcileTicked(statement, opening, txs, dups, asOf)
		if err != nil {
			rt.Fatal(err)
		}
		if got.Minor() != want.Minor() || !slices.Equal(gotU, wantU) {
			rt.Fatalf("got (%d,%v), want (%d,%v)", got.Minor(), gotU, want.Minor(), wantU)
		}
	})
}
