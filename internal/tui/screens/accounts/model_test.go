package accounts_test

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/screens/accounts"
	"github.com/0xballo/fui/internal/tui/screens/screentest"
	"github.com/0xballo/fui/internal/tui/theme"
)

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
	scr := screens.Screen(accounts.New(fx.Service, domain.LocaleENUS, theme.Default()))
	scr.SetSize(100, 30)
	return fx, scr
}

func TestAccountsLoadAndNavigate(t *testing.T) {
	fx, scr := newFixture(t)
	fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "100", OpenedOn: "2026-01-01"})
	fx.Account(t, app.AddAccountRequest{Name: "Savings", Kind: "savings", Currency: "EUR", OpeningBalance: "500", OpenedOn: "2026-01-01"})

	scr = run(t, scr, scr.Init())

	view := scr.View()
	if !strings.Contains(view, "Checking") || !strings.Contains(view, "Savings") {
		t.Fatalf("expected both accounts in view, got:\n%s", view)
	}

	scr, _ = sendKey(scr, tea.KeyDown)
	scr, cmd := sendKey(scr, tea.KeyEnter)
	scr = run(t, scr, cmd)

	detail := scr.View()
	if !strings.Contains(detail, "Savings") {
		t.Fatalf("expected the detail view of Savings, got:\n%s", detail)
	}

	scr, _ = sendKey(scr, tea.KeyEsc)
	if !strings.Contains(scr.View(), "Checking") {
		t.Fatalf("expected to be back on the list after esc")
	}
}

func TestAccountsAddForm(t *testing.T) {
	fx, scr := newFixture(t)
	scr = run(t, scr, scr.Init())

	scr, _ = sendRunes(scr, "a")
	if !strings.Contains(scr.View(), "Add account") {
		t.Fatalf("expected the add form to open, got:\n%s", scr.View())
	}

	scr, _ = sendRunes(scr, "Vault")
	scr, _ = sendKey(scr, tea.KeyEnter) // name -> kind
	scr, _ = sendRunes(scr, "cash")
	scr, _ = sendKey(scr, tea.KeyEnter) // accept the "cash" suggestion
	scr, _ = sendKey(scr, tea.KeyEnter) // kind -> currency
	scr, _ = sendRunes(scr, "EUR")
	scr, _ = sendKey(scr, tea.KeyEnter) // currency -> opening balance
	scr, _ = sendRunes(scr, "250")
	scr, _ = sendKey(scr, tea.KeyEnter) // opening balance -> opened on
	scr, cmd := sendKey(scr, tea.KeyEnter)
	scr = run(t, scr, cmd)

	if !strings.Contains(scr.View(), "Vault") {
		t.Fatalf("expected the new account in the list, got:\n%s", scr.View())
	}

	accts, err := fx.Service.ListAccounts(context.Background(), app.ListAccountsRequest{})
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	if len(accts) != 1 || accts[0].Account.Name != "Vault" {
		t.Fatalf("expected one account named Vault, got %+v", accts)
	}
}

func TestAccountsArchiveWithConfirm(t *testing.T) {
	fx, scr := newFixture(t)
	fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "0", OpenedOn: "2026-01-01"})
	scr = run(t, scr, scr.Init())

	scr, _ = sendRunes(scr, "d")
	if !strings.Contains(scr.View(), "Archive") {
		t.Fatalf("expected a confirm dialog, got:\n%s", scr.View())
	}

	scr, _ = sendRunes(scr, "n")
	if strings.Contains(scr.View(), "Archive") {
		t.Fatalf("expected the dialog to close on no")
	}

	scr, _ = sendRunes(scr, "d")
	scr, cmd := sendRunes(scr, "y")
	run(t, scr, cmd)

	accts, err := fx.Service.ListAccounts(context.Background(), app.ListAccountsRequest{})
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	if len(accts) != 0 {
		t.Fatalf("expected the archived account hidden by default, got %+v", accts)
	}
}

func TestAccountsReconcileClosesAtZeroDifference(t *testing.T) {
	fx, scr := newFixture(t)
	fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "100", OpenedOn: "2026-01-01"})
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-10", Description: "Store", Date: "2026-06-01", Status: "booked"})
	scr = run(t, scr, scr.Init())

	scr, _ = sendRunes(scr, "r")
	if !strings.Contains(scr.View(), "Reconcile account") {
		t.Fatalf("expected reconcile form, got:\n%s", scr.View())
	}

	scr, _ = sendRunes(scr, "90")
	scr, _ = sendKey(scr, tea.KeyEnter) // balance -> date
	scr, cmd := sendKey(scr, tea.KeyEnter)
	scr = run(t, scr, cmd)

	view := scr.View()
	if !strings.Contains(view, "Reconcile preview") || !strings.Contains(view, "Difference: 0.00") {
		t.Fatalf("expected zero-difference preview, got:\n%s", view)
	}

	scr, cmd = sendKey(scr, tea.KeyEnter)
	scr = run(t, scr, cmd)
	if strings.Contains(scr.View(), "Reconcile preview") {
		t.Fatalf("expected reconciliation to close and return to list, got:\n%s", scr.View())
	}

	page, err := fx.Service.ListTx(context.Background(), app.ListTxRequest{Accounts: []string{"Checking"}})
	if err != nil {
		t.Fatalf("ListTx: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].Transaction.Status != domain.StatusReconciled {
		t.Fatalf("expected transaction status reconciled, got %+v", page.Items)
	}
}

func TestAccountsReconcileNonZeroStaysPreview(t *testing.T) {
	fx, scr := newFixture(t)
	fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "100", OpenedOn: "2026-01-01"})
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-10", Description: "Store", Date: "2026-06-01", Status: "booked"})
	scr = run(t, scr, scr.Init())

	scr, _ = sendRunes(scr, "r")
	scr, _ = sendRunes(scr, "95")
	scr, _ = sendKey(scr, tea.KeyEnter)
	scr, cmd := sendKey(scr, tea.KeyEnter)
	scr = run(t, scr, cmd)

	scr, _ = sendKey(scr, tea.KeyEnter)
	view := scr.View()
	if !strings.Contains(view, "Difference must be zero") {
		t.Fatalf("expected close guard on non-zero difference, got:\n%s", view)
	}
}
