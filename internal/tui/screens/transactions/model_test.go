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

func TestTransactionsSearchEscClears(t *testing.T) {
	fx, scr := newFixture(t)
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-12.50", Description: "Coffee", Date: "2026-06-01"})
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-30", Description: "Groceries run", Date: "2026-06-02"})
	scr = run(t, scr, scr.Init())

	scr, _ = sendRunes(scr, "/")
	scr, _ = sendRunes(scr, "Coffee")
	scr, cmd := sendKey(scr, tea.KeyEnter)
	scr = run(t, scr, cmd)
	if strings.Contains(scr.View(), "Groceries run") {
		t.Fatalf("expected the search to have narrowed the list first")
	}

	scr, _ = sendRunes(scr, "/")
	scr, cmd = sendKey(scr, tea.KeyEsc)
	scr = run(t, scr, cmd)

	view := scr.View()
	if !strings.Contains(view, "Groceries run") || !strings.Contains(view, "Coffee") {
		t.Fatalf("expected esc to clear the search and show every transaction again, got:\n%s", view)
	}
	if strings.Contains(view, "Search: Coffee") {
		t.Fatalf("expected no leftover search chip after clearing, got:\n%s", view)
	}
}

func TestTransactionsSearchDebounceNarrowsList(t *testing.T) {
	fx, scr := newFixture(t)
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-12.50", Description: "Coffee", Date: "2026-06-01"})
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-30", Description: "Groceries run", Date: "2026-06-02"})
	scr = run(t, scr, scr.Init())

	scr, _ = sendRunes(scr, "/")
	if !strings.Contains(scr.View(), "Search:") {
		t.Fatalf("expected the search field to be visible while typing, got:\n%s", scr.View())
	}
	scr, cmd := sendRunes(scr, "Coffee")
	scr = run(t, scr, cmd) // waits out the real debounce timer and applies the result

	view := scr.View()
	if !strings.Contains(view, "Coffee") || strings.Contains(view, "Groceries run") {
		t.Fatalf("expected only Coffee after the debounce fires, got:\n%s", view)
	}
}

func TestTransactionsNavigationKeys(t *testing.T) {
	fx, scr := newFixture(t)
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-12.50", Description: "Coffee", Date: "2026-06-01"})
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-30", Description: "Groceries run", Date: "2026-06-02"})
	scr = run(t, scr, scr.Init())

	scr, _ = sendKey(scr, tea.KeyDown)
	scr, _ = sendKey(scr, tea.KeyUp)
	scr, _ = sendRunes(scr, "g") // top
	scr, _ = sendKey(scr, tea.KeyPgDown)
	scr, _ = sendKey(scr, tea.KeyPgUp)
	scr, _ = sendRunes(scr, "G") // bottom

	if !strings.Contains(scr.View(), "Coffee") {
		t.Fatalf("expected navigation to leave the list intact, got:\n%s", scr.View())
	}
}

func TestTransactionsChipKeysNoopWithoutChips(t *testing.T) {
	fx, scr := newFixture(t)
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-12.50", Description: "Coffee", Date: "2026-06-01"})
	scr = run(t, scr, scr.Init())

	before := scr.View()
	scr, _ = sendKey(scr, tea.KeyRight)
	scr, _ = sendKey(scr, tea.KeyLeft)
	scr, _ = sendKey(scr, tea.KeyBackspace)
	if scr.View() != before {
		t.Fatalf("expected chip navigation/removal to be a no-op without active filters, got:\n%s", scr.View())
	}
}

func TestTransactionsTitleKeyMapAndInputActive(t *testing.T) {
	_, scr := newFixture(t)
	m := scr.(*transactions.Model)

	if m.Title() != "Transactions" {
		t.Fatalf("expected title %q, got %q", "Transactions", m.Title())
	}
	if len(m.KeyMap()) == 0 {
		t.Fatalf("expected a non-empty key map")
	}
	if m.InputActive() {
		t.Fatalf("expected the list mode to not capture input")
	}

	scr2, _ := sendRunes(scr, "/")
	if !scr2.(*transactions.Model).InputActive() {
		t.Fatalf("expected search mode to capture input")
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
	scr, _ = sendKey(scr, tea.KeyEnter) // status -> kind
	scr, _ = sendKey(scr, tea.KeyEnter) // kind -> uncategorized
	scr, _ = sendKey(scr, tea.KeyEnter) // uncategorized -> sign
	scr, _ = sendKey(scr, tea.KeyEnter) // sign -> from
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

func TestTransactionsFilterByUncategorized(t *testing.T) {
	fx, scr := newFixture(t)
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-12.50", Description: "Coffee", Date: "2026-06-01"})
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-30", Description: "Groceries run", Date: "2026-06-02", Category: "Groceries"})
	scr = run(t, scr, scr.Init())

	scr, _ = sendRunes(scr, "f")
	scr, _ = sendKey(scr, tea.KeyEnter) // account -> category
	scr, _ = sendKey(scr, tea.KeyEnter) // category -> status
	scr, _ = sendKey(scr, tea.KeyEnter) // status -> kind
	scr, _ = sendKey(scr, tea.KeyEnter) // kind -> uncategorized
	scr, _ = sendRunes(scr, "yes")
	scr, _ = sendKey(scr, tea.KeyEnter) // accept suggestion
	scr, _ = sendKey(scr, tea.KeyEnter) // uncategorized -> sign
	scr, _ = sendKey(scr, tea.KeyEnter) // sign -> from
	scr, _ = sendKey(scr, tea.KeyEnter) // from -> to
	scr, _ = sendKey(scr, tea.KeyEnter) // to -> min
	scr, _ = sendKey(scr, tea.KeyEnter) // min -> max
	scr, cmd := sendKey(scr, tea.KeyEnter)
	scr = run(t, scr, cmd)

	view := scr.View()
	if !strings.Contains(view, "Coffee") || strings.Contains(view, "Groceries run") {
		t.Fatalf("expected only the uncategorized transaction, got:\n%s", view)
	}
	if !strings.Contains(view, "Uncategorized: yes") {
		t.Fatalf("expected an uncategorized chip, got:\n%s", view)
	}
}

func TestTransactionsFilterByKindAndSign(t *testing.T) {
	fx, scr := newFixture(t)
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-12.50", Description: "Coffee", Date: "2026-06-01", Kind: "payment"})
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "50", Description: "Refund", Date: "2026-06-02", Kind: "refund"})
	scr = run(t, scr, scr.Init())

	scr, _ = sendRunes(scr, "f")
	scr, _ = sendKey(scr, tea.KeyEnter) // account -> category
	scr, _ = sendKey(scr, tea.KeyEnter) // category -> status
	scr, _ = sendKey(scr, tea.KeyEnter) // status -> kind
	scr, _ = sendRunes(scr, "refund")
	scr, _ = sendKey(scr, tea.KeyEnter) // accept suggestion
	scr, _ = sendKey(scr, tea.KeyEnter) // kind -> uncategorized
	scr, _ = sendKey(scr, tea.KeyEnter) // uncategorized -> sign
	scr, _ = sendRunes(scr, "income")
	scr, _ = sendKey(scr, tea.KeyEnter) // accept suggestion
	scr, _ = sendKey(scr, tea.KeyEnter) // sign -> from
	scr, _ = sendKey(scr, tea.KeyEnter) // from -> to
	scr, _ = sendKey(scr, tea.KeyEnter) // to -> min
	scr, _ = sendKey(scr, tea.KeyEnter) // min -> max
	scr, cmd := sendKey(scr, tea.KeyEnter)
	scr = run(t, scr, cmd)

	view := scr.View()
	if !strings.Contains(view, "Refund") || strings.Contains(view, "Coffee") {
		t.Fatalf("expected only the refund transaction, got:\n%s", view)
	}
	if !strings.Contains(view, "Kind: refund") || !strings.Contains(view, "Income/Expense: income") {
		t.Fatalf("expected kind and sign chips, got:\n%s", view)
	}
}

func TestTransactionsFilterChipsRemoveOne(t *testing.T) {
	fx, scr := newFixture(t)
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-12.50", Description: "Coffee", Date: "2026-06-01"})
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-30", Description: "Groceries run", Date: "2026-06-02", Category: "Groceries"})
	scr = run(t, scr, scr.Init())

	scr, _ = sendRunes(scr, "f")
	scr, _ = sendRunes(scr, "Checking")
	scr, _ = sendKey(scr, tea.KeyEnter) // accept suggestion
	scr, _ = sendKey(scr, tea.KeyEnter) // account -> category
	scr, _ = sendRunes(scr, "Groceries")
	scr, _ = sendKey(scr, tea.KeyEnter) // accept suggestion
	scr, _ = sendKey(scr, tea.KeyEnter) // category -> status
	scr, _ = sendKey(scr, tea.KeyEnter) // status -> kind
	scr, _ = sendKey(scr, tea.KeyEnter) // kind -> uncategorized
	scr, _ = sendKey(scr, tea.KeyEnter) // uncategorized -> sign
	scr, _ = sendKey(scr, tea.KeyEnter) // sign -> from
	scr, _ = sendKey(scr, tea.KeyEnter) // from -> to
	scr, _ = sendKey(scr, tea.KeyEnter) // to -> min
	scr, _ = sendKey(scr, tea.KeyEnter) // min -> max
	scr, cmd := sendKey(scr, tea.KeyEnter)
	scr = run(t, scr, cmd)

	view := scr.View()
	if !strings.Contains(view, "Account: Checking") || !strings.Contains(view, "Category: Groceries") {
		t.Fatalf("expected filter chips in the header, got:\n%s", view)
	}
	if !strings.Contains(view, "Groceries run") || strings.Contains(view, "Coffee") {
		t.Fatalf("expected only the Groceries category after filtering, got:\n%s", view)
	}

	// Chips are in field order (account, then category): move focus to the category chip and
	// remove it, which should reload with only the account filter still active.
	scr, _ = sendKey(scr, tea.KeyRight)
	scr, cmd = sendKey(scr, tea.KeyBackspace)
	scr = run(t, scr, cmd)

	view = scr.View()
	if strings.Contains(view, "Category: Groceries") {
		t.Fatalf("expected the category chip to be removed, got:\n%s", view)
	}
	if !strings.Contains(view, "Account: Checking") {
		t.Fatalf("expected the account chip to remain, got:\n%s", view)
	}
	if !strings.Contains(view, "Coffee") {
		t.Fatalf("expected Coffee back in view after removing the category filter, got:\n%s", view)
	}
}

func TestTransactionsFooterSummary(t *testing.T) {
	fx, scr := newFixture(t)
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-12.50", Description: "Coffee", Date: "2026-06-01"})
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-30", Description: "Groceries run", Date: "2026-06-02"})
	scr = run(t, scr, scr.Init())

	view := scr.View()
	if !strings.Contains(view, "2 transactions") {
		t.Fatalf("expected a row count in the footer, got:\n%s", view)
	}
	if !strings.Contains(view, "-42.50 EUR") {
		t.Fatalf("expected the per-currency sum in the footer, got:\n%s", view)
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
