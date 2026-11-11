package transactions

import (
	"context"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/screens/screentest"
	"github.com/0xballo/fui/internal/tui/theme"
)

// run executes cmd and any cmd that Update returns in reaction, until it settles. Every tea.Cmd in
// this screen finishes all its I/O and returns one terminal message, so draining one chain is
// enough.
func run(t *testing.T, scr screens.Screen, cmd tea.Cmd) screens.Screen {
	t.Helper()
	for cmd != nil {
		msg := cmd()
		scr, cmd = scr.Update(msg)
	}
	return scr
}

// TestTrimWindow verifies that once the loaded window grows past maxWindow, the oldest-loaded
// rows farthest from the cursor (at the front of the most-recent-first slice) are dropped and the
// cursor is shifted to keep pointing at the same logical row.
func TestTrimWindow(t *testing.T) {
	m := New(nil, domain.LocaleENUS, theme.Default())
	const total = maxWindow + 50
	items := make([]app.TxView, total)
	for i := range items {
		items[i] = app.TxView{Transaction: domain.Transaction{ID: domain.TransactionID(total - i)}}
	}
	m.items = items
	m.table.SetRows(m.txRows())
	m.table.SetCursor(total - 1) // the cursor sits on the last (oldest) loaded row

	m.trimWindow()

	if len(m.items) != maxWindow {
		t.Fatalf("expected %d items after trimming, got %d", maxWindow, len(m.items))
	}
	// The dropped rows must be the oldest-loaded (front) ones, keeping the most recently fetched
	// page (the tail) intact.
	if m.items[0].Transaction.ID != domain.TransactionID(total-50) {
		t.Fatalf("expected the window to start at transaction %d, got %d", total-50, m.items[0].Transaction.ID)
	}
	if got, want := m.table.Cursor(), total-1-50; got != want {
		t.Fatalf("expected the cursor to shift to %d, got %d", want, got)
	}
}

// TestReconciledDeleteNeedsConfirmTwice verifies that deleting a reconciled transaction first asks
// the usual "delete?" question and, once confirmed, asks again before applying ConfirmReconciled.
func TestReconciledDeleteNeedsConfirmTwice(t *testing.T) {
	fx := screentest.New(t)
	fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "1000", OpenedOn: "2026-01-01"})
	tx := fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-5", Description: "Toll", Date: "2026-06-01", Status: string(domain.StatusReconciled)})

	m := New(fx.Service, domain.LocaleENUS, theme.Default())
	m.SetSize(100, 30)
	scr := run(t, screens.Screen(m), m.Init())

	scr, _ = scr.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	got := scr.(*Model)
	if got.mode != modeConfirm || got.confirmPurpose != confirmDelete {
		t.Fatalf("expected the initial delete confirm, got mode=%v purpose=%v", got.mode, got.confirmPurpose)
	}

	scr, cmd := scr.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	scr = run(t, scr, cmd)
	got = scr.(*Model)
	if got.mode != modeConfirm || got.confirmPurpose != confirmDeleteReconciled {
		t.Fatalf("expected a second confirm for the reconciled transaction, got mode=%v purpose=%v", got.mode, got.confirmPurpose)
	}

	scr, cmd = scr.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	scr = run(t, scr, cmd)
	_ = scr

	page, err := fx.Service.ListTx(context.Background(), app.ListTxRequest{})
	if err != nil {
		t.Fatalf("ListTx: %v", err)
	}
	for _, v := range page.Items {
		if v.Transaction.ID == tx.ID {
			t.Fatalf("expected the reconciled transaction to be deleted")
		}
	}
}

// TestSubmitCmdsDoNotRaceWithUpdate guards the Elm-architecture rule that a tea.Cmd returned by a
// submit* method must only use values copied out of the Model before it is returned: Bubble Tea
// runs the Cmd on its own goroutine, so anything it reads or writes directly on the live Model
// (such as m.req or m.table, both mutated by Update in reaction to ordinary key presses) races
// with Update. It runs every submit* Cmd concurrently with a goroutine that repeatedly performs
// the same mutations Update would, and must pass under -race.
func TestSubmitCmdsDoNotRaceWithUpdate(t *testing.T) {
	fx := screentest.New(t)
	fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "100", OpenedOn: "2026-01-01"})
	fx.Category(t, app.AddCategoryRequest{Name: "Groceries", Kind: string(domain.CategoryExpense)})
	tx := fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-5", Description: "Coffee", Date: "2026-06-01"})

	m := New(fx.Service, domain.LocaleENUS, theme.Default())
	m.SetSize(100, 30)
	scr := run(t, screens.Screen(m), m.Init())
	m = scr.(*Model)

	// Arm every submit* method's preconditions: a delete target, an in-progress edit form, an
	// add form, and a categorize field over the selected row.
	m.deleteTarget = tx.ID
	m.editing = &tx.ID
	m.editingTransferLeg = false
	m.amountF = components.NewAutocomplete("Amount:", "", m.th)
	m.amountF.SetValue("-6")
	m.dateF = components.NewAutocomplete("Date:", "", m.th)
	m.dateF.SetValue("2026-06-01")
	m.descF = components.NewAutocomplete("Description:", "", m.th)
	m.descF.SetValue("Coffee")
	m.categoryF = components.NewAutocomplete("Category:", "", m.th)
	m.notesF = components.NewAutocomplete("Notes:", "", m.th)
	m.statusF = components.NewAutocomplete("Status:", "", m.th)
	m.statusF.SetValue("pending")
	m.kindF = components.NewAutocomplete("Kind:", "", m.th)
	m.kindF.SetValue("payment")
	m.accountF = components.NewAutocomplete("Account:", "", m.th)
	m.accountF.SetValue("Checking")

	m.table.ToggleSelect()
	m.categorizeField = components.NewAutocomplete("New category:", "", m.th)
	m.categorizeField.SetValue("Groceries")

	cmds := []tea.Cmd{m.submitDelete(false), m.submitAdd(), m.submitEdit(false), m.submitCategorize(false)}

	var wg sync.WaitGroup
	for _, cmd := range cmds {
		wg.Add(1)
		go func(c tea.Cmd) {
			defer wg.Done()
			c()
		}(cmd)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			m.req = app.ListTxRequest{Text: "x"}
			m.table.ToggleSelect()
		}
	}()
	wg.Wait()
}
