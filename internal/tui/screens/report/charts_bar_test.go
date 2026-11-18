package report

import (
	"strconv"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/tui/components"
)

func eur(v int64) reporting.Amount { return reporting.Amount{Minor: v, Currency: "EUR"} }

func fmtPlain(a reporting.Amount) string {
	return a.Currency + " " + strconv.FormatInt(a.Minor, 10)
}

func TestBarItems(t *testing.T) {
	th := newTestModel().th
	tests := []struct {
		name string
		data any
		want []components.BarItem
	}{
		{
			"payees",
			reporting.PayeesResult{Rows: []reporting.PayeeRow{{Payee: "Shop", Total: eur(5)}}},
			[]components.BarItem{{Label: "Shop", Value: 5, DisplayValue: "EUR 5"}},
		},
		{
			"interest",
			reporting.InterestResult{Rows: []reporting.InterestRow{{Loan: "Car", Year: 2026, Interest: eur(3)}}},
			[]components.BarItem{{Label: "Car 2026", Value: 3, DisplayValue: "EUR 3"}},
		},
		{
			"budget two bars",
			reporting.BudgetResult{Lines: []reporting.BudgetLine{{Budget: "Food", Budgeted: eur(100), Spent: eur(40)}}},
			[]components.BarItem{{Label: "Food", Value: 100, DisplayValue: "EUR 100"}, {Value: 40, DisplayValue: "EUR 40"}},
		},
		{"series has none", reporting.SeriesResult{}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := barItems(th, fmtPlain, tc.data)
			if diff := cmp.Diff(tc.want, got, cmpopts.IgnoreFields(components.BarItem{}, "Style")); diff != "" {
				t.Fatalf("items mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSpentStyleTiers(t *testing.T) {
	th := newTestModel().th
	tests := []struct {
		name          string
		budget, spent int64
		want          string
	}{
		{"ok", 100, 69, "positive"},
		{"warning", 100, 70, "warning"},
		{"caution", 100, 90, "caution"},
		{"over", 100, 100, "negative"},
		{"no budget spend", 0, 5, "negative"},
		{"no budget no spend", 0, 0, "positive"},
	}
	styles := map[string]lipgloss.TerminalColor{
		"positive": th.Positive.GetForeground(), "warning": th.Warning.GetForeground(),
		"caution": th.Caution.GetForeground(), "negative": th.Negative.GetForeground(),
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := spentStyle(th, reporting.BudgetLine{Budgeted: eur(tc.budget), Spent: eur(tc.spent)}).GetForeground()
			if got != styles[tc.want] {
				t.Fatalf("style = %v, want %s %v", got, tc.want, styles[tc.want])
			}
		})
	}
}

func TestCompareBarItems(t *testing.T) {
	th := newTestModel().th
	rep := reporting.CompareReport{Rows: []reporting.CompareRowView{{Category: "Food", A: eur(7), B: eur(-2)}}}
	got := compareBarItems(th, fmtPlain, rep)
	want := []components.BarItem{{Label: "Food", Value: 7, DisplayValue: "EUR 7"}, {Value: -2, DisplayValue: "EUR -2"}}
	if diff := cmp.Diff(want, got, cmpopts.IgnoreFields(components.BarItem{}, "Style")); diff != "" {
		t.Fatalf("items mismatch (-want +got):\n%s", diff)
	}
	if got[0].Style.GetForeground() == got[1].Style.GetForeground() {
		t.Fatal("periods share a color")
	}
	total := compareBarItems(th, fmtPlain, reporting.CompareReport{Total: reporting.CompareRowView{A: eur(1), B: eur(2)}})
	if len(total) != 2 || total[0].Label != "Total" {
		t.Fatalf("total fallback = %+v", total)
	}
}

func TestBarChartViewGolden(t *testing.T) {
	m := newTestModel()
	m.result = reportResult{kind: reporting.KindBudget, data: reporting.BudgetResult{Lines: []reporting.BudgetLine{
		{Budget: "Groceries", Budgeted: eur(30000), Spent: eur(21000)},
		{Budget: "Dining", Budgeted: eur(10000), Spent: eur(12000)},
	}}}
	if m.chartView(60, 10) != "" {
		t.Fatal("bars shown on a short screen")
	}
	golden.RequireEqual(t, []byte(ansi.Strip(m.chartView(60, 24))))
}
