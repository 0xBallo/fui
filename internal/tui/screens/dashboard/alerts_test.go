package dashboard_test

import (
	"context"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/google/go-cmp/cmp"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/budgeting"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/screens/dashboard"
	"github.com/0xballo/fui/internal/tui/screens/screentest"
	"github.com/0xballo/fui/internal/tui/theme"
)

type stubBudget struct {
	st    budgeting.Status
	month string
}

func (s *stubBudget) Status(_ context.Context, month string) (budgeting.Status, error) {
	s.month = month
	return s.st, nil
}

type stubRecurring struct{ recs []app.Recurrence }

func (s stubRecurring) ListRecurrences(context.Context) ([]app.Recurrence, error) { return s.recs, nil }

func eur(t *testing.T, minor int64) domain.Money {
	t.Helper()
	c, err := domain.ParseCurrency("EUR")
	if err != nil {
		t.Fatal(err)
	}
	m, err := domain.NewMoney(minor, c)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func date(t *testing.T, y, m, d int) domain.Date {
	t.Helper()
	v, err := domain.NewDate(y, m, d)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func monthly(t *testing.T, desc string, minor int64, start domain.Date) app.Recurrence {
	t.Helper()
	rule, err := domain.NewRecurrenceRule(1, domain.RecurrenceMonths, 0)
	if err != nil {
		t.Fatal(err)
	}
	return app.Recurrence{Description: desc, Amount: eur(t, minor), Rule: rule, Start: start, NextDate: start}
}

func sampleStatus(t *testing.T) budgeting.Status {
	t.Helper()
	capLine := func(name string, state domain.BudgetState, pct int) budgeting.CapLine {
		return budgeting.CapLine{
			Budget: domain.Budget{Name: name},
			CapStatus: domain.CapStatus{
				Spent: eur(t, int64(pct)*100), Available: eur(t, 10000), Percent: pct, State: state,
			},
		}
	}
	return budgeting.Status{
		Caps: []budgeting.CapLine{
			capLine("Fun", domain.BudgetWarning, 85),
			capLine("Food", domain.BudgetOver, 120),
			capLine("Fine", domain.BudgetOK, 10),
		},
		Envelopes: []budgeting.EnvelopeLine{{
			Budget:        domain.Budget{Name: "Travel"},
			EnvelopeMonth: domain.EnvelopeMonth{Overspend: eur(t, 2500)},
		}},
	}
}

func alertsScreen(t *testing.T, b *stubBudget, r stubRecurring, ins *stubInsights, th theme.Theme) string {
	t.Helper()
	fx := screentest.New(t)
	fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "1000", OpenedOn: "2026-01-01"})
	m := dashboard.New(useCases{Service: fx.Service, Reporting: fx.NewReporting(t)}, domain.LocaleENUS, th, 1)
	m.SetAlertBudget(b)
	m.SetAlertRecurring(r)
	m.SetInsights(ins)
	m.SetSize(100, 40)
	return ansi.Strip(runBatch(t, m, m.Init()).View())
}

func TestAlertsOrderAndMapping(t *testing.T) {
	b := &stubBudget{st: sampleStatus(t)}
	r := stubRecurring{recs: []app.Recurrence{monthly(t, "Rent", -80000, date(t, 2026, 6, 27))}}
	ins := &stubInsights{items: sampleInsights()}
	view := alertsScreen(t, b, r, ins, theme.Default())

	want := []string{
		"✗ danger Food over budget",
		"✗ danger Travel overspent by 25.00",
		"! warning Fun near its cap",
		"! warning [anomaly] Grocer",
		"i info Rent -800.00 on 2026-06-27",
		"i info [subscription] Netflix",
	}
	last := -1
	for _, w := range want {
		i := strings.Index(view, w)
		if i < 0 || i < last {
			t.Fatalf("%q missing or out of order in:\n%s", w, view)
		}
		last = i
	}
	if strings.Contains(view, "Fine") {
		t.Fatalf("OK cap must not alert:\n%s", view)
	}
	if b.month != "2026-06" {
		t.Fatalf("status month = %q", b.month)
	}
	golden.RequireEqual(t, []byte(view))
}

func TestRecurrenceWindow(t *testing.T) {
	today, end := date(t, 2026, 6, 15), date(t, 2026, 6, 30)
	tests := []struct {
		name string
		rec  app.Recurrence
		want []string
	}{
		{"today", monthly(t, "A", -100, today), []string{"A -1.00 on 2026-06-15"}},
		{"end of month", monthly(t, "B", -100, end), []string{"B -1.00 on 2026-06-30"}},
		{"before today", monthly(t, "C", -100, date(t, 2026, 6, 14)), nil},
		{"next month", monthly(t, "D", -100, date(t, 2026, 7, 1)), nil},
		{"income", monthly(t, "E", 100, today), nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := dashboard.RecurrenceAlertTexts([]app.Recurrence{tc.rec}, today, end)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Fatalf("diff (-want +got):\n%s", diff)
			}
		})
	}
}

func TestAlertsDismissNeedsInsight(t *testing.T) {
	b := &stubBudget{st: sampleStatus(t)}
	fx := screentest.New(t)
	m := dashboard.New(useCases{Service: fx.Service, Reporting: fx.NewReporting(t)}, domain.LocaleENUS, theme.Default(), 1)
	m.SetAlertBudget(b)
	m.SetInsights(&stubInsights{})
	m.SetSize(100, 40)
	scr := runBatch(t, m, m.Init())
	scr, _ = sendRunes(scr, "i")
	_, cmd := sendRunes(scr, "x")
	if msg, ok := cmd().(screens.StatusMsg); !ok || !strings.Contains(msg.Text, "Only insight") {
		t.Fatalf("msg = %#v", msg)
	}
}
