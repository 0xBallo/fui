package lists_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/rules"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/screens/screentest"
	"github.com/0xballo/fui/internal/tui/screens/settings/lists"
	"github.com/0xballo/fui/internal/tui/theme"
)

func newRulesFixture(t *testing.T) (screentest.Fixture, screens.Screen) {
	t.Helper()
	fx := screentest.New(t)
	uc := rulesAdapter{Service: rules.New(fx.Store, stubBackup{}), lookups: lookups{svc: fx.Service}}
	scr := screens.Screen(lists.NewRules(uc, theme.Default()))
	scr.SetSize(100, 30)
	return fx, scr
}

func TestRulesLoadAddEditRemove(t *testing.T) {
	fx, scr := newRulesFixture(t)
	fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "0", OpenedOn: "2026-01-01"})
	fx.Category(t, app.AddCategoryRequest{Name: "Groceries", Kind: "expense"})
	scr = run(t, scr, scr.Init())

	// Add a rule.
	scr, _ = sendRunes(scr, "a")
	if !strings.Contains(scr.View(), "Add rule") {
		t.Fatalf("expected the add form to open, got:\n%s", scr.View())
	}
	scr, _ = sendRunes(scr, "Groceries rule")
	scr, _ = sendKey(scr, tea.KeyEnter) // name -> priority
	scr, _ = sendRunes(scr, "10")
	scr, _ = sendKey(scr, tea.KeyEnter) // priority -> enabled
	scr, _ = sendRunes(scr, "yes")
	scr, _ = sendKey(scr, tea.KeyEnter) // accept "yes" suggestion
	scr, _ = sendKey(scr, tea.KeyEnter) // enabled -> description contains
	scr, _ = sendRunes(scr, "SUPERMARKET")
	scr, _ = sendKey(scr, tea.KeyEnter) // -> description equals
	scr, _ = sendKey(scr, tea.KeyEnter) // -> min abs
	scr, _ = sendKey(scr, tea.KeyEnter) // -> max abs
	scr, _ = sendKey(scr, tea.KeyEnter) // -> sign
	scr, _ = sendKey(scr, tea.KeyEnter) // -> accounts
	scr, _ = sendKey(scr, tea.KeyEnter) // -> category
	scr, _ = sendRunes(scr, "Groceries")
	scr, _ = sendKey(scr, tea.KeyEnter) // accept "Groceries" suggestion
	scr, _ = sendKey(scr, tea.KeyEnter) // category -> payee
	scr, _ = sendKey(scr, tea.KeyEnter) // -> tags
	scr, cmd := sendKey(scr, tea.KeyEnter)
	scr = run(t, scr, cmd)

	if !strings.Contains(scr.View(), "Groceries rule") {
		t.Fatalf("expected the rule in the list, got:\n%s", scr.View())
	}

	// Test it against a matching description.
	scr, _ = sendRunes(scr, "t")
	if !strings.Contains(scr.View(), "Test rules") {
		t.Fatalf("expected the test form to open, got:\n%s", scr.View())
	}
	scr, _ = sendRunes(scr, "SUPERMARKET purchase")
	scr, _ = sendKey(scr, tea.KeyEnter) // text -> account
	scr, _ = sendRunes(scr, "Checking")
	scr, _ = sendKey(scr, tea.KeyEnter) // accept suggestion
	scr, _ = sendKey(scr, tea.KeyEnter) // account -> amount
	scr, _ = sendRunes(scr, "-500")
	scr, cmd = sendKey(scr, tea.KeyEnter)
	scr = run(t, scr, cmd)
	if !strings.Contains(scr.View(), "Groceries rule") {
		t.Fatalf("expected the test result to name the matching rule, got:\n%s", scr.View())
	}
	scr, _ = sendKey(scr, tea.KeyEsc)

	// Edit the rule: walk every field (all prefilled, some with a suggestion dropdown that
	// needs a second Enter to close) and bump the priority before resubmitting.
	scr, _ = sendRunes(scr, "e")
	if !strings.Contains(scr.View(), "Edit rule") {
		t.Fatalf("expected the edit form to open, got:\n%s", scr.View())
	}
	scr, _ = sendKey(scr, tea.KeyEnter) // name -> priority
	scr, _ = sendKey(scr, tea.KeyBackspace)
	scr, _ = sendKey(scr, tea.KeyBackspace)
	scr, _ = sendRunes(scr, "20")
	scr, _ = sendKey(scr, tea.KeyEnter) // priority -> enabled
	scr, _ = sendKey(scr, tea.KeyEnter) // close "yes" suggestion
	scr, _ = sendKey(scr, tea.KeyEnter) // enabled -> description contains
	scr, _ = sendKey(scr, tea.KeyEnter) // -> description equals
	scr, _ = sendKey(scr, tea.KeyEnter) // -> min abs
	scr, _ = sendKey(scr, tea.KeyEnter) // -> max abs
	scr, _ = sendKey(scr, tea.KeyEnter) // -> sign
	scr, _ = sendKey(scr, tea.KeyEnter) // close "any" suggestion
	scr, _ = sendKey(scr, tea.KeyEnter) // sign -> accounts
	scr, _ = sendKey(scr, tea.KeyEnter) // -> category
	scr, _ = sendKey(scr, tea.KeyEnter) // close "Groceries" suggestion
	scr, _ = sendKey(scr, tea.KeyEnter) // category -> payee
	scr, _ = sendKey(scr, tea.KeyEnter) // -> tags
	scr, cmd = sendKey(scr, tea.KeyEnter)
	scr = run(t, scr, cmd)
	if !strings.Contains(scr.View(), "20") {
		t.Fatalf("expected the edited priority in the list, got:\n%s", scr.View())
	}

	// Remove the rule.
	scr, cmd = sendRunes(scr, "d")
	scr = run(t, scr, cmd)
	if !strings.Contains(scr.View(), "Delete rule") {
		t.Fatalf("expected a delete confirmation, got:\n%s", scr.View())
	}
	scr, cmd = sendRunes(scr, "y")
	scr = run(t, scr, cmd)
	if strings.Contains(scr.View(), "Groceries rule") {
		t.Fatalf("expected the rule removed, got:\n%s", scr.View())
	}
}
