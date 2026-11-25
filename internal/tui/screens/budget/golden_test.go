package budget_test

import (
	"context"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/muesli/termenv"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/budgeting"
	"github.com/0xballo/fui/internal/domain"
	scr "github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/screens/budget"
	"github.com/0xballo/fui/internal/tui/theme"
)

// render builds the screen with th at 80x24 and returns its list view after loading.
func render(t *testing.T, h harness, th theme.Theme) string {
	t.Helper()
	s := scr.Screen(budget.New(h.uc(), domain.LocaleENUS, th))
	s.SetSize(80, 24)
	s = run(t, s, s.Init())
	return s.View()
}

func yearlyEnvelopeHarness(t *testing.T) harness {
	t.Helper()
	h := newHarness(t, domain.BudgetEnvelope)
	h.fx.Category(t, app.AddCategoryRequest{Name: "Groceries", Kind: "expense"})
	h.fx.Category(t, app.AddCategoryRequest{Name: "Insurance", Kind: "expense"})
	h.fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "2000", OpenedOn: "2026-01-01"})
	ctx := context.Background()
	for _, req := range []budgeting.SetRequest{
		{Name: "Groceries", Categories: []string{"Groceries"}, Amount: "0"},
		{Name: "Insurance", Categories: []string{"Insurance"}, Amount: "0", Period: "year"},
	} {
		plan, err := h.svc.PlanSet(ctx, req)
		if err != nil {
			t.Fatalf("PlanSet: %v", err)
		}
		if _, err := h.svc.ApplySet(ctx, plan); err != nil {
			t.Fatalf("ApplySet: %v", err)
		}
	}
	for _, mv := range []budgeting.MoveRequest{
		{To: "Groceries", Amount: "300", Date: "2026-06-01"},
		{To: "Insurance", Amount: "600", Date: "2026-06-01"},
	} {
		plan, err := h.svc.PlanMove(ctx, mv)
		if err != nil {
			t.Fatalf("PlanMove: %v", err)
		}
		if err := h.svc.ApplyMove(ctx, plan); err != nil {
			t.Fatalf("ApplyMove: %v", err)
		}
	}
	h.fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-240", Category: "Groceries", Date: "2026-06-05", Status: "booked"})
	return h
}

func capHarness(t *testing.T) harness {
	t.Helper()
	h := newHarness(t, domain.BudgetCap)
	h.fx.Category(t, app.AddCategoryRequest{Name: "Groceries", Kind: "expense"})
	h.fx.Category(t, app.AddCategoryRequest{Name: "Fun", Kind: "expense"})
	h.budget(t, "Groceries", "Groceries", "400")
	h.budget(t, "Fun", "Fun", "100")
	h.fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "1000", OpenedOn: "2026-01-01"})
	h.fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-150", Category: "Fun", Date: "2026-06-05", Status: "booked"})
	return h
}

// TestBudgetGoldenViews pins the 80x24 list views: the plain text must be identical across every
// theme (color is never the only signal) and free of escape codes under NO_COLOR.
func TestBudgetGoldenViews(t *testing.T) {
	prev := lipgloss.ColorProfile()
	t.Cleanup(func() { lipgloss.SetColorProfile(prev) })

	cases := []struct {
		name string
		h    func(*testing.T) harness
	}{
		{"Envelope", yearlyEnvelopeHarness},
		{"Cap", capHarness},
	}
	themes := []string{"latte", "frappe", "macchiato", "mocha", "terminal", "high-contrast"}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := c.h(t)
			lipgloss.SetColorProfile(termenv.TrueColor)
			var first string
			for _, name := range themes {
				th := theme.Resolve(theme.Config{Theme: name}, nil)
				plain := ansi.Strip(render(t, h, th))
				if first == "" {
					first = plain
				} else if plain != first {
					t.Errorf("theme %s renders differently once stripped:\n%s\nvs\n%s", name, plain, first)
				}
			}
			golden.RequireEqual(t, []byte(first))

			lipgloss.SetColorProfile(termenv.Ascii)
			v := render(t, h, theme.Default())
			if strings.Contains(v, "\x1b") {
				t.Errorf("NO_COLOR view contains escape codes: %q", v)
			}
			if ansi.Strip(v) != first {
				t.Errorf("NO_COLOR view differs from the plain golden:\n%s", v)
			}
		})
	}
}

func TestBudgetYearlyRowsAndDetail(t *testing.T) {
	h := yearlyEnvelopeHarness(t)
	s := scr.Screen(budget.New(h.uc(), domain.LocaleENUS, theme.Default()))
	s.SetSize(120, 30)
	s = run(t, s, s.Init())
	v := ansi.Strip(s.View())
	for _, want := range []string{"Insurance [year]", "Per month", "85.71"} {
		if !strings.Contains(v, want) {
			t.Errorf("wide view lacks %q:\n%s", want, v)
		}
	}

	s, _ = sendRunes(s, "j")
	if d := ansi.Strip(s.View()); !strings.Contains(d, "Insurance: year, thresholds 80/100, suggested 85.71/month") {
		t.Errorf("detail line missing for the yearly envelope:\n%s", d)
	}
}
