package dashboard_test

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

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

// run executes cmd and any cmd that Update returns in reaction, until it settles, returning the
// resulting screen. Every tea.Cmd in this screen finishes all its I/O and returns one terminal
// message, so draining one chain is enough.
func run(t *testing.T, scr screens.Screen, cmd tea.Cmd) screens.Screen {
	t.Helper()
	for cmd != nil {
		msg := cmd()
		scr, cmd = scr.Update(msg)
	}
	return scr
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
	scr := screens.Screen(dashboard.New(uc, domain.LocaleENUS, theme.Default()))
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
	if !strings.Contains(view, "Checking") {
		t.Fatalf("expected the account balance in view, got:\n%s", view)
	}
	if !strings.Contains(view, "930.00") {
		t.Fatalf("expected the net worth 930.00 EUR in view, got:\n%s", view)
	}
	if !strings.Contains(view, "Groceries") || !strings.Contains(view, "Transport") {
		t.Fatalf("expected both spending categories in view, got:\n%s", view)
	}
	if !strings.Contains(view, "not enough data") {
		t.Fatalf("expected the runway placeholder with no history, got:\n%s", view)
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
