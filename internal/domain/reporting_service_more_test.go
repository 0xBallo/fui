package domain

import (
	"errors"
	"slices"
	"testing"
)

func TestSpendingByCategoryMissingRatesAreAggregated(t *testing.T) {
	period, err := NewPeriod(date(t, 2026, 1, 1), date(t, 2026, 1, 31))
	if err != nil {
		t.Fatalf("NewPeriod: %v", err)
	}
	txs := []Transaction{
		{Date: date(t, 2026, 1, 10), Amount: money(t, -100, "USD"), Kind: KindPayment},
		{Date: date(t, 2026, 1, 20), Amount: money(t, -200, "USD"), Kind: KindPayment},
		{Date: date(t, 2026, 1, 15), Amount: money(t, -300, "CHF"), Kind: KindPayment},
	}
	_, _, err = SpendingByCategory(period, txs, cur(t, "EUR"), func(_, _ Currency, _ Date) (Rate, error) {
		return 0, &Error{Code: codeRateMissing, Message: "missing rate"}
	})
	if err == nil {
		t.Fatal("expected missing rate error")
	}
	var de *Error
	if !errors.As(err, &de) || de.Code != codeRateMissing {
		t.Fatalf("err = %v, want code %q", err, codeRateMissing)
	}
	currencies, ok := de.Details["currencies"].([]string)
	if !ok || !slices.Equal(currencies, []string{"CHF", "USD"}) {
		t.Fatalf("currencies = %v", de.Details["currencies"])
	}
	currencyDates, ok := de.Details["currency_dates"].(map[string]string)
	if !ok {
		t.Fatalf("currency_dates = %T", de.Details["currency_dates"])
	}
	if currencyDates["USD"] != "2026-01-10" || currencyDates["CHF"] != "2026-01-15" {
		t.Fatalf("currency_dates = %v", currencyDates)
	}
}

func TestSpendingByCategoryOrder(t *testing.T) {
	period, err := NewPeriod(date(t, 2026, 1, 1), date(t, 2026, 1, 31))
	if err != nil {
		t.Fatalf("NewPeriod: %v", err)
	}
	catA := CategoryID(10)
	catB := CategoryID(20)
	txs := []Transaction{
		{Date: date(t, 2026, 1, 11), Amount: money(t, -500, "EUR"), Kind: KindPayment, CategoryID: catA},
		{Date: date(t, 2026, 1, 12), Amount: money(t, -500, "EUR"), Kind: KindPayment, CategoryID: catB},
		{Date: date(t, 2026, 1, 13), Amount: money(t, -200, "EUR"), Kind: KindPayment},
	}
	rows, total, err := SpendingByCategory(period, txs, cur(t, "EUR"), nil)
	if err != nil {
		t.Fatalf("SpendingByCategory: %v", err)
	}
	if total.Minor() != 1200 {
		t.Fatalf("total = %d, want 1200", total.Minor())
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %d", len(rows))
	}
	if rows[0].CategoryID != catA || rows[1].CategoryID != catB || !rows[2].Uncategorized {
		t.Fatalf("rows order = %+v", rows)
	}
}

func TestCashFlowByMonthMissingRatesAreAggregated(t *testing.T) {
	period, err := NewPeriod(date(t, 2026, 1, 1), date(t, 2026, 2, 28))
	if err != nil {
		t.Fatalf("NewPeriod: %v", err)
	}
	txs := []Transaction{
		{Date: date(t, 2026, 1, 10), Amount: money(t, -100, "USD"), Kind: KindPayment},
		{Date: date(t, 2026, 2, 10), Amount: money(t, 200, "CHF"), Kind: KindPayment},
	}
	_, err = CashFlowByMonth(period, 1, txs, cur(t, "EUR"), func(_, _ Currency, _ Date) (Rate, error) {
		return 0, &Error{Code: codeRateMissing, Message: "missing rate"}
	})
	if err == nil {
		t.Fatal("expected missing rate error")
	}
	var de *Error
	if !errors.As(err, &de) || de.Code != codeRateMissing {
		t.Fatalf("err = %v, want code %q", err, codeRateMissing)
	}
}
