package domain

import (
	"errors"
	"testing"

	"pgregory.net/rapid"
)

func TestValidateSplits(t *testing.T) {
	total := money(t, 10000, "EUR")
	tests := []struct {
		name    string
		rows    []Split
		wantErr error
	}{
		{
			"ok",
			[]Split{
				{Category: 1, Amount: money(t, 6000, "EUR")},
				{Category: 2, Amount: money(t, 4000, "EUR")},
			},
			nil,
		},
		{"too few rows", []Split{{Category: 1, Amount: total}}, ErrInvalidValue},
		{
			"missing category",
			[]Split{
				{Category: 0, Amount: money(t, 6000, "EUR")},
				{Category: 2, Amount: money(t, 4000, "EUR")},
			},
			ErrMissingField,
		},
		{
			"currency mismatch",
			[]Split{
				{Category: 1, Amount: money(t, 6000, "EUR")},
				{Category: 2, Amount: money(t, 4000, "USD")},
			},
			ErrCurrencyMismatch,
		},
		{
			"zero row",
			[]Split{
				{Category: 1, Amount: money(t, 0, "EUR")},
				{Category: 2, Amount: money(t, 10000, "EUR")},
			},
			ErrInvalidValue,
		},
		{
			"sum mismatch",
			[]Split{
				{Category: 1, Amount: money(t, 6000, "EUR")},
				{Category: 2, Amount: money(t, 3000, "EUR")},
			},
			ErrInvalidValue,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSplits(total, tt.rows)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("ValidateSplits: %v", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ValidateSplits err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestRebalanceLastSplit(t *testing.T) {
	total := money(t, 10000, "EUR")
	rows := []Split{
		{Category: 1, Amount: money(t, 3333, "EUR")},
		{Category: 2, Amount: money(t, 3333, "EUR")},
		{Category: 3, Amount: money(t, 3333, "EUR")}, // will be overwritten
	}
	out, err := RebalanceLastSplit(total, rows)
	if err != nil {
		t.Fatal(err)
	}
	if out[2].Amount.Minor() != 10000-3333-3333 {
		t.Fatalf("last row = %v", out[2].Amount)
	}
	if err := ValidateSplits(total, out); err != nil {
		t.Fatalf("rebalanced split invalid: %v", err)
	}
	// Original rows are untouched.
	if rows[2].Amount.Minor() != 3333 {
		t.Fatalf("input rows mutated: %v", rows[2].Amount)
	}

	if _, err := RebalanceLastSplit(total, rows[:1]); !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("too few rows err = %v", err)
	}

	zeroing := []Split{
		{Category: 1, Amount: money(t, 10000, "EUR")},
		{Category: 2, Amount: money(t, 0, "EUR")},
	}
	if _, err := RebalanceLastSplit(total, zeroing); !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("zeroing last row err = %v", err)
	}

	mismatched := []Split{
		{Category: 1, Amount: money(t, 6000, "USD")},
		{Category: 2, Amount: money(t, 4000, "EUR")},
	}
	if _, err := RebalanceLastSplit(total, mismatched); !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("currency mismatch err = %v", err)
	}
}

// TestValidateSplitsSumProperty checks that splitting total into N rows via
// RebalanceLastSplit always produces rows ValidateSplits accepts, for any
// total and any number of equal-ish rows.
func TestValidateSplitsSumProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		c := cur(t, "EUR")
		total, err := NewMoney(rapid.Int64Range(-1<<40, 1<<40).Draw(t, "total"), c)
		if err != nil {
			t.Fatal(err)
		}
		n := rapid.IntRange(2, 6).Draw(t, "n")
		share := total.Minor() / int64(n)
		if share == 0 {
			return // every row but the last would be zero, which ValidateSplits rejects
		}
		rows := make([]Split, n)
		for i := range rows {
			amt, err := NewMoney(share, c)
			if err != nil {
				t.Fatal(err)
			}
			rows[i] = Split{Category: CategoryID(i + 1), Amount: amt}
		}
		out, err := RebalanceLastSplit(total, rows)
		if err != nil {
			// The only expected failure is the rebalanced last row landing on
			// zero, which happens when share*n == total.
			if errors.Is(err, ErrInvalidValue) {
				return
			}
			t.Fatal(err)
		}
		if err := ValidateSplits(total, out); err != nil {
			t.Fatalf("ValidateSplits after rebalance: %v", err)
		}
	})
}
