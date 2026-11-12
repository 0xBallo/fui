package budget_test

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/budgeting"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
	scr "github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/screens/budget"
	"github.com/0xballo/fui/internal/tui/screens/screentest"
	"github.com/0xballo/fui/internal/tui/theme"
)

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

// harness wires a budgeting.Service over the same store screentest.Fixture built, so tests
// exercise the real use cases: screentest covers accounts/categories/transactions, this covers
// budgets and moves.
type harness struct {
	fx  screentest.Fixture
	svc *budgeting.Service
	now time.Time
	eur domain.Currency
}

func newHarness(t *testing.T, mode domain.BudgetMode) harness {
	t.Helper()
	fx := screentest.New(t)
	eur, err := domain.ParseCurrency("EUR")
	if err != nil {
		t.Fatalf("ParseCurrency: %v", err)
	}
	now := time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC)
	svc := budgeting.New(fx.Store, fixedClock{now: now}, budgeting.Config{
		BaseCurrency: eur, Location: time.UTC, Modules: module.Default(), Mode: mode,
	})
	return harness{fx: fx, svc: svc, now: now, eur: eur}
}

// budget creates a budget covering category and fails the test on error.
func (h harness) budget(t *testing.T, name, category, amount string) {
	t.Helper()
	ctx := context.Background()
	plan, err := h.svc.PlanSet(ctx, budgeting.SetRequest{Name: name, Categories: []string{category}, Amount: amount})
	if err != nil {
		t.Fatalf("PlanSet(%q): %v", name, err)
	}
	if _, err := h.svc.ApplySet(ctx, plan); err != nil {
		t.Fatalf("ApplySet(%q): %v", name, err)
	}
}

func newFixture(t *testing.T, h harness) scr.Screen {
	t.Helper()
	s := scr.Screen(budget.New(h.svc, domain.LocaleENUS, theme.Default()))
	s.SetSize(120, 30)
	return s
}

func run(t *testing.T, s scr.Screen, cmd tea.Cmd) scr.Screen {
	t.Helper()
	for cmd != nil {
		msg := cmd()
		s, cmd = s.Update(msg)
	}
	return s
}

func sendKey(s scr.Screen, k tea.KeyType) (scr.Screen, tea.Cmd) {
	return s.Update(tea.KeyMsg{Type: k})
}

func sendRunes(s scr.Screen, str string) (scr.Screen, tea.Cmd) {
	return s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(str)})
}

func TestBudgetCapStatus(t *testing.T) {
	h := newHarness(t, domain.BudgetCap)
	h.fx.Category(t, app.AddCategoryRequest{Name: "Groceries", Kind: "expense"})
	h.budget(t, "Groceries", "Groceries", "400")
	h.fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "1000", OpenedOn: "2026-01-01"})
	h.fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-150", Category: "Groceries", Date: "2026-06-05", Status: "booked"})

	s := newFixture(t, h)
	s = run(t, s, s.Init())

	view := s.View()
	for _, want := range []string{"Groceries", "400", "150", "250", "OK"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected %q in view, got:\n%s", want, view)
		}
	}
}

func TestBudgetSetAmount(t *testing.T) {
	h := newHarness(t, domain.BudgetCap)
	h.fx.Category(t, app.AddCategoryRequest{Name: "Groceries", Kind: "expense"})
	h.budget(t, "Groceries", "Groceries", "400")

	s := newFixture(t, h)
	s = run(t, s, s.Init())

	s, _ = sendRunes(s, "s")
	if !strings.Contains(s.View(), "Set amount") {
		t.Fatalf("expected the set-amount form to open, got:\n%s", s.View())
	}
	s, _ = sendRunes(s, "500")
	s, cmd := sendKey(s, tea.KeyEnter)
	s = run(t, s, cmd)

	if !strings.Contains(s.View(), "500") {
		t.Fatalf("expected the new amount in view, got:\n%s", s.View())
	}
}

func TestBudgetNavigateMonthsAndCopy(t *testing.T) {
	h := newHarness(t, domain.BudgetCap)
	h.fx.Category(t, app.AddCategoryRequest{Name: "Groceries", Kind: "expense"})
	h.budget(t, "Groceries", "Groceries", "400")

	s := newFixture(t, h)
	s = run(t, s, s.Init())
	if !strings.Contains(s.View(), "2026-06") {
		t.Fatalf("expected the current month, got:\n%s", s.View())
	}

	s, cmd := sendKey(s, tea.KeyRight)
	s = run(t, s, cmd)
	if !strings.Contains(s.View(), "2026-07") {
		t.Fatalf("expected next month 2026-07, got:\n%s", s.View())
	}

	s, cmd = sendKey(s, tea.KeyLeft)
	s = run(t, s, cmd)
	if !strings.Contains(s.View(), "2026-06") {
		t.Fatalf("expected back to 2026-06, got:\n%s", s.View())
	}

	s, cmd = sendKey(s, tea.KeyRight)
	s = run(t, s, cmd)
	s, _ = sendRunes(s, "c")
	if !strings.Contains(s.View(), "Copy previous month") {
		t.Fatalf("expected a confirm dialog, got:\n%s", s.View())
	}
	s, cmd = sendRunes(s, "y")
	run(t, s, cmd)

	st, err := h.svc.Status(context.Background(), "2026-07")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(st.Caps) != 1 || st.Caps[0].Amount.Minor() != 40000 {
		t.Fatalf("expected the copied cap amount, got %+v", st.Caps)
	}
}

func TestBudgetEnvelopeAssignAndMove(t *testing.T) {
	h := newHarness(t, domain.BudgetEnvelope)
	h.fx.Category(t, app.AddCategoryRequest{Name: "Groceries", Kind: "expense"})
	h.fx.Category(t, app.AddCategoryRequest{Name: "Transport", Kind: "expense"})
	h.budget(t, "Groceries", "Groceries", "0")
	h.budget(t, "Transport", "Transport", "0")
	h.fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "1000", OpenedOn: "2026-01-01"})

	s := newFixture(t, h)
	s = run(t, s, s.Init())

	view := s.View()
	if !strings.Contains(view, "To assign") {
		t.Fatalf("expected the to-assign footer, got:\n%s", view)
	}

	s, _ = sendRunes(s, "a")
	if !strings.Contains(s.View(), "Assign from unassigned pool") {
		t.Fatalf("expected the assign form to open, got:\n%s", s.View())
	}
	s, _ = sendRunes(s, "200")
	s, cmd := sendKey(s, tea.KeyEnter)
	s = run(t, s, cmd)
	if !strings.Contains(s.View(), "200") {
		t.Fatalf("expected the assigned balance in view, got:\n%s", s.View())
	}

	s, _ = sendRunes(s, "m") // cursor is still on Groceries, the assigned envelope
	if !strings.Contains(s.View(), "Move between envelopes") {
		t.Fatalf("expected the move form to open, got:\n%s", s.View())
	}
	s, _ = sendRunes(s, "Transport")
	s, _ = sendKey(s, tea.KeyEnter) // accept the "Transport" suggestion
	s, _ = sendKey(s, tea.KeyEnter) // to -> amount
	s, _ = sendRunes(s, "50")
	s, cmd = sendKey(s, tea.KeyEnter)
	run(t, s, cmd)

	st, err := h.svc.Status(context.Background(), "")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	var groceries, transport budgeting.EnvelopeLine
	for _, l := range st.Envelopes {
		switch l.Budget.Name {
		case "Groceries":
			groceries = l
		case "Transport":
			transport = l
		}
	}
	if groceries.Balance.Minor() != 15000 {
		t.Fatalf("expected Groceries balance 150, got %v", groceries.Balance)
	}
	if transport.Balance.Minor() != 5000 {
		t.Fatalf("expected Transport balance 50, got %v", transport.Balance)
	}
}

func TestBudgetScreenAccessors(t *testing.T) {
	h := newHarness(t, domain.BudgetCap)
	h.fx.Category(t, app.AddCategoryRequest{Name: "Groceries", Kind: "expense"})
	h.budget(t, "Groceries", "Groceries", "400")
	s := newFixture(t, h)

	if got := s.Title(); got != "Budget" {
		t.Fatalf("Title() = %q, want %q", got, "Budget")
	}
	if len(s.KeyMap()) == 0 {
		t.Fatal("expected a non-empty key map")
	}
	if s.InputActive() {
		t.Fatal("expected InputActive false before opening a form")
	}

	s = run(t, s, s.Init())
	s, _ = sendRunes(s, "s")
	if !s.InputActive() {
		t.Fatal("expected InputActive true while the set-amount form is open")
	}
	s, _ = sendKey(s, tea.KeyEsc)
	if s.InputActive() {
		t.Fatal("expected InputActive false after esc closes the form")
	}
}

func TestBudgetCopyCancel(t *testing.T) {
	h := newHarness(t, domain.BudgetCap)
	h.fx.Category(t, app.AddCategoryRequest{Name: "Groceries", Kind: "expense"})
	h.budget(t, "Groceries", "Groceries", "400")

	s := newFixture(t, h)
	s = run(t, s, s.Init())

	s, _ = sendRunes(s, "c")
	if !strings.Contains(s.View(), "Copy previous month") {
		t.Fatalf("expected the confirm dialog, got:\n%s", s.View())
	}
	s, _ = sendRunes(s, "n")
	if strings.Contains(s.View(), "Copy previous month") {
		t.Fatal("expected the dialog to close on no")
	}
}

func TestBudgetCapWarningAndOverStates(t *testing.T) {
	h := newHarness(t, domain.BudgetCap)
	h.fx.Category(t, app.AddCategoryRequest{Name: "Groceries", Kind: "expense"})
	h.fx.Category(t, app.AddCategoryRequest{Name: "Fun", Kind: "expense"})
	h.budget(t, "Groceries", "Groceries", "100")
	h.budget(t, "Fun", "Fun", "100")
	h.fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "1000", OpenedOn: "2026-01-01"})
	h.fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-90", Category: "Groceries", Date: "2026-06-05", Status: "booked"})
	h.fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-150", Category: "Fun", Date: "2026-06-05", Status: "booked"})

	s := newFixture(t, h)
	s = run(t, s, s.Init())

	view := s.View()
	if !strings.Contains(view, "WARNING") {
		t.Fatalf("expected a WARNING state, got:\n%s", view)
	}
	if !strings.Contains(view, "OVER") {
		t.Fatalf("expected an OVER state, got:\n%s", view)
	}
}

func TestBudgetAssignMoveIgnoredInCapMode(t *testing.T) {
	h := newHarness(t, domain.BudgetCap)
	h.fx.Category(t, app.AddCategoryRequest{Name: "Groceries", Kind: "expense"})
	h.budget(t, "Groceries", "Groceries", "400")

	s := newFixture(t, h)
	s = run(t, s, s.Init())

	s, _ = sendRunes(s, "a")
	if s.InputActive() {
		t.Fatal("expected assign to be a no-op in cap mode")
	}
	s, _ = sendRunes(s, "m")
	if s.InputActive() {
		t.Fatal("expected move to be a no-op in cap mode")
	}
}
