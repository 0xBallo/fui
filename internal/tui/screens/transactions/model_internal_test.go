package transactions

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
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
