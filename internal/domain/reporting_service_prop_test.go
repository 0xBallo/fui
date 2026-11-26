package domain

import (
	"testing"

	"pgregory.net/rapid"
)

func TestSpendingTotalsIgnoreSplitsProperty(t *testing.T) {
	eur, err := ParseCurrency("EUR")
	if err != nil {
		t.Fatal(err)
	}
	day := gd(t, "2026-03-05")
	period, err := NewPeriod(gd(t, "2026-03-01"), gd(t, "2026-03-31"))
	if err != nil {
		t.Fatal(err)
	}
	rates := func(Currency, Currency, Date) (Rate, error) { return NewRate(RateScale) }

	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(1, 8).Draw(rt, "n")
		var rows []Transaction
		var outflow int64
		for range n {
			amount := rapid.Int64Range(2, 100_000).Draw(rt, "amount")
			outflow += amount
			parts := rapid.IntRange(1, 4).Draw(rt, "parts")
			remaining := amount
			for i := range parts {
				part := remaining
				if i < parts-1 && remaining > 1 {
					part = rapid.Int64Range(1, remaining-1).Draw(rt, "part")
				}
				remaining -= part
				if part == 0 {
					continue
				}
				m, mErr := NewMoney(-part, eur)
				if mErr != nil {
					rt.Fatal(mErr)
				}
				cat := CategoryID(rapid.IntRange(0, 4).Draw(rt, "cat"))
				rows = append(rows, Transaction{Date: day, Amount: m, CategoryID: cat, Status: StatusBooked, Kind: KindPayment})
			}
		}
		totals, total, sErr := SpendingByCategory(period, rows, eur, rates)
		if sErr != nil {
			rt.Fatal(sErr)
		}
		var sum int64
		for _, ct := range totals {
			sum += ct.Total.Minor()
		}
		if sum != outflow || total.Minor() != outflow {
			rt.Fatalf("category sum=%d total=%d outflow=%d", sum, total.Minor(), outflow)
		}
	})
}
