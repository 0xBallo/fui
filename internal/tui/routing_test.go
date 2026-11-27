package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/tui/screens"
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
	if !strings.Contains(plainView(m), "Late arrival") {
		t.Fatalf("Transactions did not receive its result while inactive:\n%s", plainView(m))
	}
}

func TestSwitchScreenFilterRouting(t *testing.T) {
	// Screens keep their state across tab switches, so every case gets its own root.
	seeded := func(t *testing.T) tea.Model {
		t.Helper()
		fx, m := newRoot(t)
		fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "1000", OpenedOn: "2026-01-01"})
		fx.Category(t, app.AddCategoryRequest{Name: "Groceries"})
		fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-30", Description: "Cheese", Category: "Groceries", Date: "2026-06-10"})
		fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-9", Description: "Cinema", Date: "2026-06-11"})
		return m
	}

	filter := &app.ListTxRequest{Categories: []string{"Groceries"}}

	t.Run("receiver applies the filter", func(t *testing.T) {
		m, cmd := seeded(t).Update(screens.SwitchScreenMsg{Target: screens.TabTransactions, Filter: filter})
		m = run(t, m, cmd)
		v := plainView(m)
		if !strings.Contains(v, "Cheese") || strings.Contains(v, "Cinema") || !strings.Contains(v, "Category: Groceries") {
			t.Fatalf("filter not applied:\n%s", v)
		}
	})

	t.Run("screen without receiver ignores the filter", func(t *testing.T) {
		m, cmd := seeded(t).Update(screens.SwitchScreenMsg{Target: screens.TabAccounts, Filter: filter})
		if cmd == nil {
			t.Fatal("switching must still init the target screen")
		}
		m = run(t, m, cmd)
		if v := plainView(m); strings.Contains(v, "Category: Groceries") {
			t.Fatalf("filter leaked into Accounts:\n%s", v)
		}
	})

	t.Run("nil filter keeps the plain switch", func(t *testing.T) {
		m, cmd := seeded(t).Update(screens.SwitchScreenMsg{Target: screens.TabTransactions})
		m = run(t, m, cmd)
		if v := plainView(m); !strings.Contains(v, "Cinema") {
			t.Fatalf("unfiltered list expected:\n%s", v)
		}
	})
}
