package report

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

func TestMergeFilterRowWins(t *testing.T) {
	base := app.ListTxRequest{
		Categories: []string{"Transport"}, Payees: []string{"Metro"}, Accounts: []string{"Checking"},
		Sign: domain.TxSignIncome, MinAmount: "5", From: "2020-01-01",
	}
	cases := []struct {
		name string
		row  app.ListTxRequest
		want app.ListTxRequest
	}{
		{
			"row category replaces",
			app.ListTxRequest{From: "2026-06-01", To: "2026-06-30", Categories: []string{"Groceries"}},
			app.ListTxRequest{
				From: "2026-06-01", To: "2026-06-30", Categories: []string{"Groceries"}, Payees: []string{"Metro"},
				Accounts: []string{"Checking"}, Sign: domain.TxSignIncome, MinAmount: "5",
			},
		},
		{
			"row uncategorized drops categories",
			app.ListTxRequest{Uncategorized: true, Sign: domain.TxSignExpense},
			app.ListTxRequest{
				Uncategorized: true, Payees: []string{"Metro"}, Accounts: []string{"Checking"},
				Sign: domain.TxSignExpense, MinAmount: "5",
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, mergeFilter(base, c.row)); diff != "" {
				t.Fatalf("mergeFilter mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFilterSummaryAndActive(t *testing.T) {
	if filterActive(app.ListTxRequest{Sign: domain.TxSignAny}) {
		t.Fatal("an any-sign filter must not count as active")
	}
	f := app.ListTxRequest{Tags: []string{"a", "b"}, Uncategorized: true, MaxAmount: "9"}
	got := filterSummary(f, "tag", "week")
	want := "by tag · per week · Filter: tag=a|b, max=9, uncategorized"
	if got != want {
		t.Fatalf("filterSummary = %q, want %q", got, want)
	}
	if filterSummary(app.ListTxRequest{}, "", "") != "" {
		t.Fatal("an empty filter must have no summary")
	}
}
