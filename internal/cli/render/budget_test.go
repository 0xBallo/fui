package render

import (
	"strings"
	"testing"

	"github.com/0xballo/fui/internal/app/budgeting"
	"github.com/0xballo/fui/internal/domain"
)

func eur(t *testing.T, minor int64) domain.Money {
	t.Helper()
	cur, err := domain.ParseCurrency("EUR")
	if err != nil {
		t.Fatal(err)
	}
	m, err := domain.NewMoney(minor, cur)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func testStatus(t *testing.T, mode domain.BudgetMode) budgeting.Status {
	t.Helper()
	d, err := domain.ParseDate("2026-11-10")
	if err != nil {
		t.Fatal(err)
	}
	p, err := domain.MonthPeriod(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	monthly := domain.Budget{Name: "Spesa", Mode: mode, Amount: eur(t, 40000), Carry: true}
	yearly := domain.Budget{
		Name: "Viaggi", Mode: mode, Amount: eur(t, 120000), Period: domain.BudgetYearly,
		Thresholds: domain.BudgetThresholds{Warn: 70, Over: 110},
	}
	st := budgeting.Status{Period: p, Mode: mode, Uncovered: eur(t, 500)}
	if mode == domain.BudgetEnvelope {
		st.Envelopes = []budgeting.EnvelopeLine{
			{Budget: yearly, SuggestedMonthly: eur(t, 60000), EnvelopeMonth: domain.EnvelopeMonth{
				CarryIn: eur(t, 0), Assigned: eur(t, 120000), MovesIn: eur(t, 0), MovesOut: eur(t, 0),
				Spent: eur(t, 0), Planned: eur(t, 0), Balance: eur(t, 120000), Overspend: eur(t, 0),
			}},
			{Budget: monthly, EnvelopeMonth: domain.EnvelopeMonth{
				CarryIn: eur(t, 0), Assigned: eur(t, 100), MovesIn: eur(t, 0), MovesOut: eur(t, 0),
				Spent: eur(t, 200), Planned: eur(t, 0), Balance: eur(t, -100), Overspend: eur(t, 100),
			}},
		}
		st.ToAssign = domain.Unassigned{Liquidity: eur(t, 1000), Envelopes: eur(t, 400), ToAssign: eur(t, 600)}
		return st
	}
	capLine := func(b domain.Budget, spent int64, state domain.BudgetState) budgeting.CapLine {
		return budgeting.CapLine{Budget: b, CapStatus: domain.CapStatus{
			Period: p, Amount: b.Amount, CarryIn: eur(t, 0), Available: b.Amount, Spent: eur(t, spent),
			Planned: eur(t, 0), Remaining: eur(t, 0), Percent: 90, State: state,
		}}
	}
	st.Caps = []budgeting.CapLine{capLine(yearly, 1, domain.BudgetOver), capLine(monthly, 2, domain.BudgetWarning)}
	return st
}

func TestWriteBudgetStatus(t *testing.T) {
	tests := []struct {
		name string
		mode domain.BudgetMode
		f    Format
		want []string
	}{
		{"cap text", domain.BudgetCap, Table, []string{"PERIOD", "WARN", "OVER", "year", "70", "110", "OVER", "WARNING", "Uncovered"}},
		{"cap json", domain.BudgetCap, JSON, []string{`"period":"year"`, `"warn_at":70`, `"over_at":110`, `"warn_at":80`, `"state":"OVER"`}},
		{"cap csv", domain.BudgetCap, CSV, []string{"NAME,PERIOD", "Viaggi,year", "WARNING"}},
		{"envelope text", domain.BudgetEnvelope, Table, []string{"SUGGESTED_MONTHLY", "600,00", "To assign", "OVER"}},
		{"envelope json", domain.BudgetEnvelope, JSON, []string{`"suggested_monthly":60000`, `"suggested_monthly":0`, `"period":"year"`, `"to_assign"`}},
		{"envelope csv", domain.BudgetEnvelope, CSV, []string{"SUGGESTED_MONTHLY", "year"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var b strings.Builder
			if err := WriteBudgetStatus(&b, tc.f, domain.LocaleITIT, false, testStatus(t, tc.mode)); err != nil {
				t.Fatal(err)
			}
			for _, w := range tc.want {
				if !strings.Contains(b.String(), w) {
					t.Errorf("output missing %q:\n%s", w, b.String())
				}
			}
		})
	}
}

func TestWriteBudgetListShowsPeriodAndThresholds(t *testing.T) {
	budgets := []domain.Budget{
		{Name: "Spesa", Mode: domain.BudgetCap, Amount: eur(t, 40000), Carry: true},
		{
			Name: "Viaggi", Mode: domain.BudgetCap, Amount: eur(t, 120000), Period: domain.BudgetYearly,
			Thresholds: domain.BudgetThresholds{Warn: 70, Over: 110},
		},
	}
	for _, tc := range []struct {
		f    Format
		want []string
	}{
		{Table, []string{"PERIOD", "WARN", "OVER", "month", "year", "80", "100", "70", "110"}},
		{JSON, []string{`"period":"month"`, `"period":"year"`, `"warn_at":70`, `"over_at":110`, `"warn_at":80`}},
		{CSV, []string{"NAME,MODE,PERIOD,AMOUNT,CARRY,WARN,OVER", "Viaggi,cap,year"}},
	} {
		var b strings.Builder
		if err := WriteBudgetList(&b, tc.f, domain.LocaleENUS, budgets); err != nil {
			t.Fatal(err)
		}
		for _, w := range tc.want {
			if !strings.Contains(b.String(), w) {
				t.Errorf("format %v missing %q:\n%s", tc.f, w, b.String())
			}
		}
	}
	var b strings.Builder
	if err := WriteBudget(&b, JSON, domain.LocaleENUS, budgets[0]); err != nil || !strings.Contains(b.String(), `"name":"Spesa"`) {
		t.Errorf("WriteBudget = %q, %v", b.String(), err)
	}
}

func TestWriteBudgetMoveAndCopy(t *testing.T) {
	d, _ := domain.ParseDate("2026-11-10")
	for _, f := range []Format{Table, JSON, CSV} {
		var b strings.Builder
		if err := WriteBudgetMove(&b, f, domain.LocaleENUS, d, "A", "B", eur(t, 100)); err != nil || !strings.Contains(b.String(), "2026-11-10") {
			t.Errorf("move %v = %q, %v", f, b.String(), err)
		}
		b.Reset()
		if err := WriteBudgetCopyResult(&b, f, 2, 1); err != nil || b.Len() == 0 {
			t.Errorf("copy %v = %q, %v", f, b.String(), err)
		}
	}
}
