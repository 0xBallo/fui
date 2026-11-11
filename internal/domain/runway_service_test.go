package domain

import (
	"errors"
	"testing"

	"pgregory.net/rapid"
)

func TestMedianInt64(t *testing.T) {
	tests := []struct {
		name   string
		values []int64
		want   int64
	}{
		{"odd", []int64{50, 10, 30}, 30},
		{"even", []int64{10, 30, 20, 40}, 25},
		{"even half-away", []int64{1, 2}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := medianInt64(tt.values)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
	if _, err := medianInt64(nil); !errors.Is(err, ErrNotEnoughData) {
		t.Fatalf("empty err = %v", err)
	}
}

func TestLastCompleteMonthsWindowBoundaries(t *testing.T) {
	tests := []struct {
		name     string
		asOf     Date
		startDay int
		wantFrom Date
		wantTo   Date
	}{
		{
			name:     "start day 27",
			asOf:     date(t, 2026, 11, 11),
			startDay: 27,
			wantFrom: date(t, 2025, 10, 27),
			wantTo:   date(t, 2026, 10, 26),
		},
		{
			name:     "start day 31",
			asOf:     date(t, 2026, 3, 1),
			startDay: 31,
			wantFrom: date(t, 2025, 2, 28),
			wantTo:   date(t, 2026, 2, 27),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := lastCompleteMonthsWindow(tt.asOf, tt.startDay, 12)
			if err != nil {
				t.Fatal(err)
			}
			if got.From() != tt.wantFrom || got.To() != tt.wantTo {
				t.Fatalf("window = [%v,%v], want [%v,%v]", got.From(), got.To(), tt.wantFrom, tt.wantTo)
			}
		})
	}
}

func TestRunway(t *testing.T) {
	asOf := date(t, 2026, 11, 15)
	balances := []RunwayBalance{
		{Kind: AccountChecking, Balance: money(t, 300_000, "EUR")},
		{Kind: AccountCash, Balance: money(t, 50_000, "EUR")},
		{Kind: AccountCreditCard, Balance: money(t, -40_000, "EUR")},
		{Kind: AccountInvestment, Balance: money(t, 900_000, "EUR")},
	}
	movements := []RunwayMovement{
		{Date: date(t, 2026, 8, 10), Amount: money(t, -100_000, "EUR"), Kind: KindPayment},
		{Date: date(t, 2026, 9, 10), Amount: money(t, -120_000, "EUR"), Kind: KindPayment},
		{Date: date(t, 2026, 10, 10), Amount: money(t, -80_000, "EUR"), Kind: KindPayment},
		{Date: date(t, 2026, 10, 11), Amount: money(t, -70_000, "EUR"), Kind: KindTransfer},
	}
	got, err := Runway(asOf, 1, balances, movements)
	if err != nil {
		t.Fatal(err)
	}
	if got.Liquidity.Minor() != 310_000 {
		t.Fatalf("liquidity = %d", got.Liquidity.Minor())
	}
	if got.ReferenceMonthlySpending.Minor() != 100_000 {
		t.Fatalf("spending = %d", got.ReferenceMonthlySpending.Minor())
	}
	if got.Months != 3 || got.Days != 3 {
		t.Fatalf("duration = %d months %d days", got.Months, got.Days)
	}
	if got.ZeroDate != date(t, 2027, 2, 16) {
		t.Fatalf("zero date = %v", got.ZeroDate)
	}
}

func TestRunwayErrors(t *testing.T) {
	asOf := date(t, 2026, 11, 15)
	_, err := Runway(asOf, 1, []RunwayBalance{{Kind: AccountCash, Balance: money(t, 100_000, "EUR")}}, []RunwayMovement{
		{Date: date(t, 2026, 10, 1), Amount: money(t, -50_000, "EUR"), Kind: KindPayment},
		{Date: date(t, 2026, 9, 1), Amount: money(t, -50_000, "EUR"), Kind: KindPayment},
	})
	if !errors.Is(err, ErrNotEnoughData) {
		t.Fatalf("not enough data err = %v", err)
	}
	_, err = Runway(asOf, 1, []RunwayBalance{{Kind: AccountCash, Balance: money(t, 1, "EUR")}}, []RunwayMovement{
		{Date: date(t, 2026, 10, 1), Amount: money(t, -1, "USD"), Kind: KindPayment},
		{Date: date(t, 2026, 9, 1), Amount: money(t, -1, "USD"), Kind: KindPayment},
		{Date: date(t, 2026, 8, 1), Amount: money(t, -1, "USD"), Kind: KindPayment},
	})
	if !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("currency mismatch err = %v", err)
	}
}

func TestRunwayMonotonicInLiquidityProperty(t *testing.T) {
	asOf := date(t, 2026, 11, 15)
	baseMovements := []RunwayMovement{
		{Date: date(t, 2026, 8, 10), Amount: money(t, -100_000, "EUR"), Kind: KindPayment},
		{Date: date(t, 2026, 9, 10), Amount: money(t, -100_000, "EUR"), Kind: KindPayment},
		{Date: date(t, 2026, 10, 10), Amount: money(t, -100_000, "EUR"), Kind: KindPayment},
	}
	rapid.Check(t, func(t *rapid.T) {
		base := rapid.Int64Range(0, 2_000_000).Draw(t, "base")
		extra := rapid.Int64Range(0, 2_000_000).Draw(t, "extra")
		low, err := Runway(asOf, 1, []RunwayBalance{{Kind: AccountChecking, Balance: money(t, base, "EUR")}}, baseMovements)
		if err != nil {
			t.Fatal(err)
		}
		high, err := Runway(asOf, 1, []RunwayBalance{{Kind: AccountChecking, Balance: money(t, base+extra, "EUR")}}, baseMovements)
		if err != nil {
			t.Fatal(err)
		}
		lowDays := low.Months*30 + low.Days
		highDays := high.Months*30 + high.Days
		if highDays < lowDays {
			t.Fatalf("non-monotonic: low=%d high=%d", lowDays, highDays)
		}
	})
}
