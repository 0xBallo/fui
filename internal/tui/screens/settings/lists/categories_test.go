package lists_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/screens/screentest"
	"github.com/0xballo/fui/internal/tui/screens/settings/lists"
	"github.com/0xballo/fui/internal/tui/theme"
)

func newCategoriesFixture(t *testing.T) (screentest.Fixture, screens.Screen) {
	t.Helper()
	fx := screentest.New(t)
	scr := screens.Screen(lists.NewCategories(fx.Service, theme.Default()))
	scr.SetSize(100, 30)
	return fx, scr
}

func TestCategoriesLoadAddRenameRemove(t *testing.T) {
	fx, scr := newCategoriesFixture(t)
	fx.Category(t, app.AddCategoryRequest{Name: "Groceries", Kind: "expense"})
	scr = run(t, scr, scr.Init())

	if !strings.Contains(scr.View(), "Groceries") {
		t.Fatalf("expected Groceries in view, got:\n%s", scr.View())
	}

	// Add a category.
	scr, _ = sendRunes(scr, "a")
	if !strings.Contains(scr.View(), "Add category") {
		t.Fatalf("expected the add form to open, got:\n%s", scr.View())
	}
	scr, _ = sendRunes(scr, "Transport")
	scr, _ = sendKey(scr, tea.KeyEnter) // name -> kind
	scr, _ = sendRunes(scr, "expense")
	scr, _ = sendKey(scr, tea.KeyEnter) // accept the "expense" suggestion
	scr, cmd := sendKey(scr, tea.KeyEnter)
	scr = run(t, scr, cmd)

	if !strings.Contains(scr.View(), "Transport") {
		t.Fatalf("expected Transport after add, got:\n%s", scr.View())
	}

	// Rename the Groceries category.
	scr, _ = sendKey(scr, tea.KeyDown)
	scr, _ = sendRunes(scr, "e")
	if !strings.Contains(scr.View(), "Rename category") {
		t.Fatalf("expected the rename form to open, got:\n%s", scr.View())
	}
	for range "Groceries" {
		scr, _ = sendKey(scr, tea.KeyBackspace)
	}
	scr, _ = sendRunes(scr, "Food")
	scr, cmd = sendKey(scr, tea.KeyEnter)
	scr = run(t, scr, cmd)
	if !strings.Contains(scr.View(), "Food") {
		t.Fatalf("expected Food after rename, got:\n%s", scr.View())
	}

	// Remove it (unused, so no reassignment is needed).
	scr, cmd = sendRunes(scr, "d")
	scr = run(t, scr, cmd)
	if !strings.Contains(scr.View(), "Delete category") {
		t.Fatalf("expected a delete confirmation, got:\n%s", scr.View())
	}
	scr, cmd = sendRunes(scr, "y")
	scr = run(t, scr, cmd)
	if strings.Contains(scr.View(), "Food") {
		t.Fatalf("expected Food to be removed, got:\n%s", scr.View())
	}
}

func TestCategoriesRemoveInUseReassigns(t *testing.T) {
	fx, scr := newCategoriesFixture(t)
	fx.Category(t, app.AddCategoryRequest{Name: "Groceries", Kind: "expense"})
	fx.Category(t, app.AddCategoryRequest{Name: "Food", Kind: "expense"})
	acct := fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "100", OpenedOn: "2026-01-01"})
	fx.Tx(t, app.AddTxRequest{Account: acct.Name, Date: "2026-06-01", Amount: "-10", Category: "Groceries"})
	scr = run(t, scr, scr.Init())

	// Groceries sorts first; select and delete it.
	scr, cmd := sendRunes(scr, "d")
	scr = run(t, scr, cmd)
	if !strings.Contains(scr.View(), "Reassign") {
		t.Fatalf("expected a reassign prompt for the in-use category, got:\n%s", scr.View())
	}
	scr, _ = sendRunes(scr, "Food")
	scr, _ = sendKey(scr, tea.KeyEnter) // accept the "Food" suggestion
	scr, cmd = sendKey(scr, tea.KeyEnter)
	scr = run(t, scr, cmd)
	scr, cmd = sendRunes(scr, "y")
	scr = run(t, scr, cmd)

	if strings.Contains(scr.View(), "Groceries") {
		t.Fatalf("expected Groceries removed after reassignment, got:\n%s", scr.View())
	}
}
