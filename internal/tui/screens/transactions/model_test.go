package transactions_test

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/screens/screentest"
	"github.com/0xballo/fui/internal/tui/screens/transactions"
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
	fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "1000", OpenedOn: "2026-01-01"})
	fx.Category(t, app.AddCategoryRequest{Name: "Groceries", Kind: string(domain.CategoryExpense)})
	scr := screens.Screen(transactions.New(fx.Service, domain.LocaleENUS, theme.Default()))
	scr.SetSize(120, 30)
	return fx, scr
}

func TestTransactionsLoadAndNavigate(t *testing.T) {
	fx, scr := newFixture(t)
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-12.50", Description: "Coffee", Date: "2026-06-01"})
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-30", Description: "Groceries run", Date: "2026-06-02", Category: "Groceries"})

	scr = run(t, scr, scr.Init())

	view := scr.View()
	if !strings.Contains(view, "Coffee") || !strings.Contains(view, "Groceries run") {
		t.Fatalf("expected both transactions in view, got:\n%s", view)
	}
}

func TestTransactionsAddForm(t *testing.T) {
	fx, scr := newFixture(t)
	scr = run(t, scr, scr.Init())

	scr, _ = sendRunes(scr, "a")
	if !strings.Contains(scr.View(), "Add transaction") {
		t.Fatalf("expected the add form to open, got:\n%s", scr.View())
	}

	scr, _ = sendRunes(scr, "Checking")
	scr, _ = sendKey(scr, tea.KeyEnter) // accept suggestion
	scr, _ = sendKey(scr, tea.KeyEnter) // account -> amount
	scr, _ = sendRunes(scr, "-42.50")
	scr, _ = sendKey(scr, tea.KeyEnter) // amount -> date
	scr, _ = sendRunes(scr, "2026-06-10")
	scr, _ = sendKey(scr, tea.KeyEnter) // date -> description
	scr, _ = sendRunes(scr, "Train ticket")
	scr, _ = sendKey(scr, tea.KeyEnter) // description -> category
	scr, _ = sendKey(scr, tea.KeyEnter) // category -> notes (left empty)
	scr, _ = sendKey(scr, tea.KeyEnter) // notes -> status (left empty)
	scr, _ = sendKey(scr, tea.KeyEnter) // status -> kind (left empty)
	scr, cmd := sendKey(scr, tea.KeyEnter)
	scr = run(t, scr, cmd)

	if !strings.Contains(scr.View(), "Train ticket") {
		t.Fatalf("expected the new transaction in the list, got:\n%s", scr.View())
	}

	page, err := fx.Service.ListTx(context.Background(), app.ListTxRequest{})
	if err != nil {
		t.Fatalf("ListTx: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].Transaction.Description != "Train ticket" {
		t.Fatalf("expected one transaction named Train ticket, got %+v", page.Items)
	}
}

func TestTransactionsEditForm(t *testing.T) {
	fx, scr := newFixture(t)
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-12.50", Description: "Coffee", Date: "2026-06-01"})
	scr = run(t, scr, scr.Init())

	scr, cmd := sendKey(scr, tea.KeyEnter) // open the edit form for the selected row
	scr = run(t, scr, cmd)
	if !strings.Contains(scr.View(), "Edit transaction") {
		t.Fatalf("expected the edit form to open, got:\n%s", scr.View())
	}

	for range 20 {
		scr, _ = sendKey(scr, tea.KeyBackspace)
	}
	scr, _ = sendRunes(scr, "-9.99")
	scr, _ = sendKey(scr, tea.KeyEnter) // amount -> date
	scr, _ = sendKey(scr, tea.KeyEnter) // date -> description (unchanged)
	scr, _ = sendRunes(scr, " shop")
	scr, _ = sendKey(scr, tea.KeyEnter) // description -> category
	scr, _ = sendKey(scr, tea.KeyEnter) // category -> notes
	scr, _ = sendKey(scr, tea.KeyEnter) // notes -> status (prefilled "booked", opens its dropdown)
	scr, _ = sendKey(scr, tea.KeyEnter) // accept the "booked" suggestion
	scr, _ = sendKey(scr, tea.KeyEnter) // status -> kind (prefilled "payment", opens its dropdown)
	scr, _ = sendKey(scr, tea.KeyEnter) // accept the "payment" suggestion
	scr, cmd = sendKey(scr, tea.KeyEnter)
	run(t, scr, cmd)

	page, err := fx.Service.ListTx(context.Background(), app.ListTxRequest{})
	if err != nil {
		t.Fatalf("ListTx: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("expected one transaction, got %+v", page.Items)
	}
	got := page.Items[0].Transaction
	if got.Description != "Coffee shop" {
		t.Fatalf("expected description %q, got %q", "Coffee shop", got.Description)
	}
	if domain.FormatMachine(got.Amount) != "-9.99" {
		t.Fatalf("expected amount -9.99, got %s", domain.FormatMachine(got.Amount))
	}
}

func TestTransactionsDeleteWithConfirm(t *testing.T) {
	fx, scr := newFixture(t)
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-12.50", Description: "Coffee", Date: "2026-06-01"})
	scr = run(t, scr, scr.Init())

	scr, _ = sendRunes(scr, "d")
	if !strings.Contains(scr.View(), "Delete") {
		t.Fatalf("expected a confirm dialog, got:\n%s", scr.View())
	}

	scr, _ = sendRunes(scr, "n")
	if strings.Contains(scr.View(), "Delete transaction") {
		t.Fatalf("expected the dialog to close on no")
	}

	scr, _ = sendRunes(scr, "d")
	scr, cmd := sendRunes(scr, "y")
	run(t, scr, cmd)

	page, err := fx.Service.ListTx(context.Background(), app.ListTxRequest{})
	if err != nil {
		t.Fatalf("ListTx: %v", err)
	}
	if len(page.Items) != 0 {
		t.Fatalf("expected the transaction to be deleted, got %+v", page.Items)
	}
}

func TestTransactionsSearch(t *testing.T) {
	fx, scr := newFixture(t)
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-12.50", Description: "Coffee", Date: "2026-06-01"})
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-30", Description: "Groceries run", Date: "2026-06-02"})
	scr = run(t, scr, scr.Init())

	scr, _ = sendRunes(scr, "/")
	scr, _ = sendRunes(scr, "Coffee")
	scr, cmd := sendKey(scr, tea.KeyEnter)
	scr = run(t, scr, cmd)

	view := scr.View()
	if !strings.Contains(view, "Coffee") || strings.Contains(view, "Groceries run") {
		t.Fatalf("expected only Coffee after searching, got:\n%s", view)
	}
}

func TestTransactionsFilterByCategory(t *testing.T) {
	fx, scr := newFixture(t)
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-12.50", Description: "Coffee", Date: "2026-06-01"})
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-30", Description: "Groceries run", Date: "2026-06-02", Category: "Groceries"})
	scr = run(t, scr, scr.Init())

	scr, _ = sendRunes(scr, "f")
	if !strings.Contains(scr.View(), "Filter transactions") {
		t.Fatalf("expected the filter form to open, got:\n%s", scr.View())
	}
	scr, _ = sendRunes(scr, "Checking")
	scr, _ = sendKey(scr, tea.KeyEnter) // accept suggestion
	scr, _ = sendKey(scr, tea.KeyEnter) // account -> category
	scr, _ = sendRunes(scr, "Groceries")
	scr, _ = sendKey(scr, tea.KeyEnter) // accept suggestion
	scr, _ = sendKey(scr, tea.KeyEnter) // category -> status
	scr, _ = sendKey(scr, tea.KeyEnter) // status -> from
	scr, _ = sendKey(scr, tea.KeyEnter) // from -> to
	scr, _ = sendKey(scr, tea.KeyEnter) // to -> min
	scr, _ = sendKey(scr, tea.KeyEnter) // min -> max
	scr, cmd := sendKey(scr, tea.KeyEnter)
	scr = run(t, scr, cmd)

	view := scr.View()
	if !strings.Contains(view, "Groceries run") || strings.Contains(view, "Coffee") {
		t.Fatalf("expected only the Groceries category after filtering, got:\n%s", view)
	}

	scr, cmd = sendKey(scr, tea.KeyCtrlR)
	scr = run(t, scr, cmd)
	view = scr.View()
	if !strings.Contains(view, "Coffee") {
		t.Fatalf("expected the filter to reset, got:\n%s", view)
	}
}

func TestTransactionsBulkCategorize(t *testing.T) {
	fx, scr := newFixture(t)
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-12.50", Description: "Coffee", Date: "2026-06-01"})
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-8", Description: "Snack", Date: "2026-06-02"})
	scr = run(t, scr, scr.Init())

	scr, _ = sendKey(scr, tea.KeySpace) // select the first (most recent) row
	scr, _ = sendKey(scr, tea.KeyDown)
	scr, _ = sendKey(scr, tea.KeySpace) // select the second row too
	scr, _ = sendRunes(scr, "c")
	scr, _ = sendRunes(scr, "Groceries")
	scr, _ = sendKey(scr, tea.KeyEnter) // accept suggestion
	scr, cmd := sendKey(scr, tea.KeyEnter)
	run(t, scr, cmd)

	page, err := fx.Service.ListTx(context.Background(), app.ListTxRequest{})
	if err != nil {
		t.Fatalf("ListTx: %v", err)
	}
	for _, v := range page.Items {
		if v.Category != "Groceries" {
			t.Fatalf("expected every transaction categorized as Groceries, got %+v", v)
		}
	}
}
