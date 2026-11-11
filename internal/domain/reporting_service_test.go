package domain

import (
	"errors"
	"testing"
)

func TestSpendingByCategory(t *testing.T) {
	period, _ := NewPeriod(date(t, 2026, 11, 1), date(t, 2026, 11, 30))
	txs := []Transaction{
		{Date: date(t, 2026, 11, 2), Amount: money(t, -10_000, "EUR"), Kind: KindPayment, CategoryID: 1},
		{Date: date(t, 2026, 11, 3), Amount: money(t, -5_000, "USD"), Kind: KindPayment},
		{Date: date(t, 2026, 11, 4), Amount: money(t, -7_000, "EUR"), Kind: KindTransfer, CategoryID: 2},
		{Date: date(t, 2026, 11, 5), Amount: money(t, 5_000, "EUR"), Kind: KindRefund, CategoryID: 1},
		{Date: date(t, 2026, 10, 31), Amount: money(t, -9_999, "EUR"), Kind: KindPayment, CategoryID: 3},
	}
	rows, total, err := SpendingByCategory(period, txs, cur(t, "EUR"), fixedRateLookup(map[string]Rate{
		"USD->EUR@2026-11-03": rate(t, 900_000_000),
	}))
	if err != nil {
		t.Fatal(err)
	}
	if total.Minor() != 14_500 {
		t.Fatalf("total = %d", total.Minor())
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d", len(rows))
	}
	if rows[0].CategoryID != 1 || rows[0].Total.Minor() != 10_000 {
		t.Fatalf("row0 = %+v", rows[0])
	}
	if !rows[1].Uncategorized || rows[1].Total.Minor() != 4_500 {
		t.Fatalf("row1 = %+v", rows[1])
	}
}

func TestCashFlowByMonth(t *testing.T) {
	period, _ := NewPeriod(date(t, 2026, 11, 1), date(t, 2027, 1, 31))
	txs := []Transaction{
		{Date: date(t, 2026, 11, 26), Amount: money(t, -10_000, "EUR"), Kind: KindPayment},
		{Date: date(t, 2026, 11, 27), Amount: money(t, 15_000, "EUR"), Kind: KindInterest},
		{Date: date(t, 2026, 12, 31), Amount: money(t, -4_000, "EUR"), Kind: KindPayment},
		{Date: date(t, 2027, 1, 1), Amount: money(t, -3_000, "EUR"), Kind: KindInvestment},
	}
	tests := []struct {
		name       string
		startDay   int
		wantFirst  Period
		wantSecond Period
	}{
		{
			name:       "month start 27",
			startDay:   27,
			wantFirst:  mustPeriod(t, date(t, 2026, 10, 27), date(t, 2026, 11, 26)),
			wantSecond: mustPeriod(t, date(t, 2026, 11, 27), date(t, 2026, 12, 26)),
		},
		{
			name:       "month start 31",
			startDay:   31,
			wantFirst:  mustPeriod(t, date(t, 2026, 10, 31), date(t, 2026, 11, 29)),
			wantSecond: mustPeriod(t, date(t, 2026, 11, 30), date(t, 2026, 12, 30)),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows, err := CashFlowByMonth(period, tt.startDay, txs, cur(t, "EUR"), nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) < 2 {
				t.Fatalf("rows = %d", len(rows))
			}
			if rows[0].Period != tt.wantFirst || rows[1].Period != tt.wantSecond {
				t.Fatalf("periods = [%v..%v], [%v..%v]", rows[0].Period.From(), rows[0].Period.To(), rows[1].Period.From(), rows[1].Period.To())
			}
		})
	}
}

func TestReportingServiceErrors(t *testing.T) {
	period, _ := NewPeriod(date(t, 2026, 1, 1), date(t, 2026, 1, 31))
	_, _, err := SpendingByCategory(period, []Transaction{{Date: date(t, 2026, 1, 1), Amount: money(t, -1, "USD"), Kind: KindPayment}}, cur(t, "EUR"), nil)
	if !errors.Is(err, ErrMissingField) {
		t.Fatalf("missing lookup err = %v", err)
	}
	_, err = CashFlowByMonth(period, 0, nil, cur(t, "EUR"), nil)
	if !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("invalid month start err = %v", err)
	}
}

func fixedRateLookup(values map[string]Rate) RateLookup {
	return func(from Currency, to Currency, at Date) (Rate, error) {
		key := from.Code() + "->" + to.Code() + "@" + at.String()
		value, ok := values[key]
		if !ok {
			return 0, errors.New("rate not found")
		}
		return value, nil
	}
}

func mustPeriod(t tb, from, to Date) Period {
	t.Helper()
	p, err := NewPeriod(from, to)
	if err != nil {
		t.Fatalf("NewPeriod(%v,%v): %v", from, to, err)
	}
	return p
}
