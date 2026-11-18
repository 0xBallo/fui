package dashboard_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/screens/dashboard"
	"github.com/0xballo/fui/internal/tui/screens/screentest"
	"github.com/0xballo/fui/internal/tui/theme"
)

// useCases composes a Fixture's Service with its reporting use cases to satisfy
// dashboard.UseCases.
type useCases struct {
	*app.Service
	screentest.Reporting
}

// run executes cmd and every cmd that Update returns in reaction (expanding tea.Batch), until it
// settles, returning the resulting screen. Every tea.Cmd in this screen finishes all its I/O and
// returns one terminal message.
func run(t *testing.T, scr screens.Screen, cmd tea.Cmd) screens.Screen {
	t.Helper()
	if cmd == nil {
		return scr
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			scr = run(t, scr, c)
		}
		return scr
	}
	scr, next := scr.Update(msg)
	return run(t, scr, next)
}

func sendKey(scr screens.Screen, k tea.KeyType) (screens.Screen, tea.Cmd) {
	return scr.Update(tea.KeyMsg{Type: k})
}

func sendRunes(scr screens.Screen, s string) (screens.Screen, tea.Cmd) {
	return scr.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
}

func newFixture(t *testing.T) (screentest.Fixture, screens.Screen) {
	t.Helper()
	fx := screentest.New(t)
	uc := useCases{Service: fx.Service, Reporting: fx.NewReporting(t)}
	scr := screens.Screen(dashboard.New(uc, domain.LocaleENUS, theme.Default(), 1))
	scr.SetSize(100, 30)
	return fx, scr
}

func TestDashboardLoadsSummary(t *testing.T) {
	fx, scr := newFixture(t)
	fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "1000", OpenedOn: "2026-01-01"})
	fx.Category(t, app.AddCategoryRequest{Name: "Groceries", Kind: "expense"})
	fx.Category(t, app.AddCategoryRequest{Name: "Transport", Kind: "expense"})
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-50", Description: "Supermarket", Category: "Groceries", Date: "2026-06-10"})
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-20", Description: "Bus pass", Category: "Transport", Date: "2026-06-11"})

	scr = run(t, scr, scr.Init())

	view := scr.View()
	if !strings.Contains(view, "June 2026") {
		t.Fatalf("expected the cycle header in view, got:\n%s", view)
	}
	if !strings.Contains(ansi.Strip(view), "net of loans 930.00") {
		t.Fatalf("expected the net worth 930.00 in view, got:\n%s", view)
	}
}

func TestDashboardQuickAdd(t *testing.T) {
	fx, scr := newFixture(t)
	fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "1000", OpenedOn: "2026-01-01"})
	fx.Category(t, app.AddCategoryRequest{Name: "Groceries", Kind: "expense"})

	scr = run(t, scr, scr.Init())

	scr, _ = sendRunes(scr, "a")
	scr, _ = sendRunes(scr, "Checking")
	scr, _ = sendKey(scr, tea.KeyEnter) // accept the "Checking" suggestion
	scr, _ = sendKey(scr, tea.KeyEnter) // advance to Amount
	scr, _ = sendRunes(scr, "-15.50")
	scr, _ = sendKey(scr, tea.KeyEnter) // advance to Description
	scr, _ = sendRunes(scr, "Corner shop")
	scr, _ = sendKey(scr, tea.KeyEnter) // advance to Category
	scr, _ = sendRunes(scr, "Groceries")
	scr, _ = sendKey(scr, tea.KeyEnter) // accept the "Groceries" suggestion
	scr, cmd := sendKey(scr, tea.KeyEnter)
	scr = run(t, scr, cmd)

	page, err := fx.Service.ListTx(context.Background(), app.ListTxRequest{})
	if err != nil {
		t.Fatalf("ListTx: %v", err)
	}
	var found bool
	for _, v := range page.Items {
		if v.Transaction.Description == "Corner shop" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected the quick-added transaction to be stored, got %+v", page.Items)
	}

	view := scr.View()
	if !strings.Contains(view, "984.50") {
		t.Fatalf("expected the updated balance 984.50 EUR in view, got:\n%s", view)
	}
}

func TestDashboardCycleKeys(t *testing.T) {
	fx, scr := newFixture(t)
	fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "1000", OpenedOn: "2024-03-01"})
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-50", Description: "Old", Date: "2024-03-05"})
	scr = run(t, scr, scr.Init())

	header := func() string { return strings.Split(ansi.Strip(scr.View()), "\n")[0] }
	if h := header(); !strings.Contains(h, "June 2026 · month") || strings.Contains(h, "▶") {
		t.Fatalf("current month: next must be disabled, header %q", h)
	}
	scr, cmd := sendRunes(scr, "l")
	scr = run(t, scr, cmd)
	if h := header(); !strings.Contains(h, "June 2026") {
		t.Fatalf("next at the current period must be a no-op, header %q", h)
	}
	scr, cmd = sendRunes(scr, "h")
	scr = run(t, scr, cmd)
	if h := header(); !strings.Contains(h, "◀ May 2026 · month ▶") {
		t.Fatalf("after h, header %q", h)
	}
	scr, cmd = sendRunes(scr, "p")
	scr = run(t, scr, cmd)
	if h := header(); !strings.Contains(h, "Q2 2026 · quarter") {
		t.Fatalf("after p, header %q", h)
	}
	scr, cmd = sendRunes(scr, "p")
	scr = run(t, scr, cmd)
	if h := header(); !strings.Contains(h, "2026 · year") {
		t.Fatalf("after p, header %q", h)
	}
	scr, cmd = sendRunes(scr, "p")
	scr = run(t, scr, cmd)
	if h := header(); !strings.Contains(h, "since 2024-03-05 · lifetime") {
		t.Fatalf("after p, header %q", h)
	}
}

func TestDashboardNetWorthFollowsPeriod(t *testing.T) {
	fx, scr := newFixture(t)
	fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "1000", OpenedOn: "2026-01-01"})
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-100", Description: "June", Date: "2026-06-10"})
	scr = run(t, scr, scr.Init())
	if !strings.Contains(ansi.Strip(scr.View()), "net of loans 900.00") {
		t.Fatalf("expected 900.00 now:\n%s", scr.View())
	}
	scr, cmd := sendRunes(scr, "h")
	scr = run(t, scr, cmd)
	if !strings.Contains(ansi.Strip(scr.View()), "net of loans 1,000.00") {
		t.Fatalf("expected 1,000.00 at the end of May:\n%s", scr.View())
	}
}

func TestDashboardGolden(t *testing.T) {
	for _, size := range [][2]int{{80, 20}, {140, 30}} {
		fx := screentest.New(t)
		fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "1000", OpenedOn: "2026-01-01"})
		uc := useCases{Service: fx.Service, Reporting: fx.NewReporting(t)}
		scr := screens.Screen(dashboard.New(uc, domain.LocaleENUS, theme.Default(), 1))
		scr.SetSize(size[0], size[1])
		scr = run(t, scr, scr.Init())
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			golden.RequireEqual(t, []byte(ansi.Strip(scr.View())))
		})
	}
}
