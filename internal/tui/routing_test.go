package tui_test

import (
	"strings"
	"testing"

	"github.com/0xballo/fui/internal/app"
)

// TestAsyncResultReachesInactiveTab starts the Transactions load, switches to Accounts before the
// result arrives, and checks the result is still applied to Transactions: without targeted
// routing it would be handed to Accounts and dropped.
func TestAsyncResultReachesInactiveTab(t *testing.T) {
	fx, m := newRoot(t)
	fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "1000", OpenedOn: "2026-01-01"})
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-30", Description: "Late arrival", Date: "2026-06-10"})

	m, load := sendRunes(m, "2")
	if load == nil {
		t.Fatal("switching to Transactions should start a load")
	}
	m, _ = sendRunes(m, "3") // Accounts becomes active; its own load is never delivered
	m, _ = m.Update(load())

	m, _ = sendRunes(m, "2") // the reload this starts is deliberately not run
	if !strings.Contains(m.View(), "Late arrival") {
		t.Fatalf("Transactions did not receive its result while inactive:\n%s", m.View())
	}
}
