package report

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/domain"
)

func TestDrillFilter(t *testing.T) {
	eur := func(v int64) reporting.Amount { return reporting.Amount{Minor: v, Currency: "EUR"} }
	grouped := func(by string, keys ...string) reporting.GroupedResult {
		g := reporting.GroupedResult{Period: "2026-01-01..2026-03-31", GroupBy: by}
		for _, k := range keys {
			g.Rows = append(g.Rows, reporting.GroupedRow{Key: k, Total: eur(1)})
		}
		return g
	}
	series := reporting.SeriesResult{Period: "2026-01-01..2026-03-31", Points: []reporting.Point{
		{Date: "2026-01-31", Amount: eur(1)}, {Date: "2026-02-28", Amount: eur(2)},
	}}
	cases := []struct {
		name   string
		result reportResult
		acct   string
		row    int
		want   app.ListTxRequest
		ok     bool
	}{
		{
			"uncategorized",
			reportResult{kind: reporting.KindSpending, data: grouped("category", reporting.UncategorizedCategoryName)},
			"", 0,
			app.ListTxRequest{From: "2026-01-01", To: "2026-03-31", Uncategorized: true, Sign: domain.TxSignExpense},
			true,
		},
		{
			"payee",
			reportResult{kind: reporting.KindSpending, data: grouped("payee", "Shop")},
			"", 0,
			app.ListTxRequest{From: "2026-01-01", To: "2026-03-31", Payees: []string{"Shop"}, Sign: domain.TxSignExpense},
			true,
		},
		{
			"no payee",
			reportResult{kind: reporting.KindSpending, data: grouped("payee", reporting.NoPayeeLabel)},
			"", 0,
			app.ListTxRequest{From: "2026-01-01", To: "2026-03-31", Sign: domain.TxSignExpense},
			true,
		},
		{
			"account",
			reportResult{kind: reporting.KindSpending, data: grouped("account", "Checking")},
			"", 0,
			app.ListTxRequest{From: "2026-01-01", To: "2026-03-31", Accounts: []string{"Checking"}, Sign: domain.TxSignExpense},
			true,
		},
		{
			"tag",
			reportResult{kind: reporting.KindSpending, data: grouped("tag", "trip")},
			"", 0,
			app.ListTxRequest{From: "2026-01-01", To: "2026-03-31", Tags: []string{"trip"}, Sign: domain.TxSignExpense},
			true,
		},
		{
			"untagged",
			reportResult{kind: reporting.KindSpending, data: grouped("tag", reporting.UntaggedLabel)},
			"", 0,
			app.ListTxRequest{From: "2026-01-01", To: "2026-03-31", Sign: domain.TxSignExpense},
			true,
		},
		{
			"month bucket",
			reportResult{kind: reporting.KindCashFlow, data: grouped("month", "2026-01-01", "2026-02-01")},
			"", 0,
			app.ListTxRequest{From: "2026-01-01", To: "2026-01-31"},
			true,
		},
		{
			"last bucket",
			reportResult{kind: reporting.KindCashFlow, data: grouped("month", "2026-01-01", "2026-02-01")},
			"", 1,
			app.ListTxRequest{From: "2026-02-01", To: "2026-03-31"},
			true,
		},
		{"beyond rows", reportResult{kind: reporting.KindCashFlow, data: grouped("month", "2026-01-01")}, "", 5, app.ListTxRequest{}, false},
		{"bad period", reportResult{kind: reporting.KindCashFlow, data: reporting.GroupedResult{Period: "x"}}, "", 0, app.ListTxRequest{}, false},
		{
			"first point",
			reportResult{kind: reporting.KindNetWorth, data: series},
			"", 0,
			app.ListTxRequest{From: "2026-01-01", To: "2026-01-31"},
			true,
		},
		{
			"second point with account",
			reportResult{kind: reporting.KindBalance, data: series},
			"Checking", 1,
			app.ListTxRequest{From: "2026-02-01", To: "2026-02-28", Accounts: []string{"Checking"}},
			true,
		},
		{"net worth point", reportResult{kind: reporting.KindNetWorth, data: reporting.NetWorthResult{
			Period: series.Period, Points: []reporting.NetWorthPoint{{Date: "2026-01-31", Total: eur(1)}},
		}}, "", 0, app.ListTxRequest{From: "2026-01-01", To: "2026-01-31"}, true},
		{"payees total", reportResult{kind: reporting.KindPayees, data: reporting.PayeesResult{
			Period: series.Period, Rows: []reporting.PayeeRow{{Payee: "A"}},
		}}, "", 1, app.ListTxRequest{From: "2026-01-01", To: "2026-03-31"}, true},
		{"forecast", reportResult{kind: reporting.KindForecast, data: reporting.ForecastResult{}}, "", 0, app.ListTxRequest{}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := newTestModel()
			m.result, m.accountText = c.result, c.acct
			got, ok := m.drillFilter(c.row)
			if ok != c.ok {
				t.Fatalf("ok = %v, want %v", ok, c.ok)
			}
			if ok {
				if diff := cmp.Diff(c.want, got); diff != "" {
					t.Fatalf("filter mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestDrillHelpers(t *testing.T) {
	if _, err := dayBefore("nope"); err == nil {
		t.Fatal("dayBefore accepted a bad date")
	}
	if _, err := dayAfter("nope"); err == nil {
		t.Fatal("dayAfter accepted a bad date")
	}
	m := newTestModel()
	m.result = reportResult{kind: reporting.KindForecast, data: reporting.ForecastResult{}}
	if m.drill() != nil || m.drillCompare() != nil {
		t.Fatal("drill must do nothing without rows")
	}
	if _, ok := m.compareFilter(0); ok {
		t.Fatal("compareFilter accepted an unloaded comparison")
	}
	if got := groupTitle(""); got != "Group" {
		t.Fatalf("groupTitle = %q", got)
	}
}
