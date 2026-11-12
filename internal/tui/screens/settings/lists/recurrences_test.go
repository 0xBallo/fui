package lists_test

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/recurring"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/screens/screentest"
	"github.com/0xballo/fui/internal/tui/screens/settings/lists"
	"github.com/0xballo/fui/internal/tui/theme"
)

func newRecurrencesFixture(t *testing.T) (screentest.Fixture, screens.Screen) {
	t.Helper()
	fx := screentest.New(t)
	enabled, err := module.Validate([]module.ID{module.Core, module.Recurring})
	if err != nil {
		t.Fatalf("module.Validate: %v", err)
	}
	cfg := recurring.Config{Locale: domain.LocaleENUS, Location: time.UTC}
	uc := recurringAdapter{
		Service: recurring.New(fx.Store, fixedClock{now: time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC)}, stubBackup{}, cfg),
		lookups: lookups{svc: fx.Service},
	}
	scr := screens.Screen(lists.NewRecurrences(uc, enabled, domain.LocaleENUS, theme.Default()))
	scr.SetSize(100, 30)
	return fx, scr
}

func TestRecurrencesSubmitErrorShownInline(t *testing.T) {
	fx, scr := newRecurrencesFixture(t)
	fx.Category(t, app.AddCategoryRequest{Name: "Rent", Kind: "expense"})
	scr = run(t, scr, scr.Init())

	// Add a recurrence against an account that does not exist: the add form stays open with
	// the error visible instead of silently discarding it.
	scr, _ = sendRunes(scr, "a")
	scr, _ = sendRunes(scr, "Nonexistent")
	scr, _ = sendKey(scr, tea.KeyEnter) // account -> category
	scr, _ = sendRunes(scr, "Rent")
	scr, _ = sendKey(scr, tea.KeyEnter) // accept suggestion
	scr, _ = sendKey(scr, tea.KeyEnter) // category -> description
	scr, _ = sendRunes(scr, "Monthly rent")
	scr, _ = sendKey(scr, tea.KeyEnter) // -> amount
	scr, _ = sendRunes(scr, "-850")
	scr, _ = sendKey(scr, tea.KeyEnter) // -> every
	scr, _ = sendRunes(scr, "1")
	scr, _ = sendKey(scr, tea.KeyEnter) // -> unit
	scr, _ = sendRunes(scr, "months")
	scr, _ = sendKey(scr, tea.KeyEnter) // accept suggestion
	scr, _ = sendKey(scr, tea.KeyEnter) // unit -> day of month
	scr, _ = sendRunes(scr, "1")
	scr, _ = sendKey(scr, tea.KeyEnter) // -> start
	scr, _ = sendRunes(scr, "2026-07-01")
	scr, _ = sendKey(scr, tea.KeyEnter) // -> end
	scr, cmd := sendKey(scr, tea.KeyEnter)
	scr = run(t, scr, cmd)

	if !strings.Contains(scr.View(), "Add recurrence") {
		t.Fatalf("expected to stay on the add form after a failed submit, got:\n%s", scr.View())
	}
	if !strings.Contains(scr.View(), "Error:") {
		t.Fatalf("expected the submit error to be shown inline, got:\n%s", scr.View())
	}
}

func TestRecurrencesLoadAddEditRemove(t *testing.T) {
	fx, scr := newRecurrencesFixture(t)
	fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "0", OpenedOn: "2026-01-01"})
	fx.Category(t, app.AddCategoryRequest{Name: "Rent", Kind: "expense"})
	scr = run(t, scr, scr.Init())

	// Add a recurrence.
	scr, _ = sendRunes(scr, "a")
	if !strings.Contains(scr.View(), "Add recurrence") {
		t.Fatalf("expected the add form to open, got:\n%s", scr.View())
	}
	scr, _ = sendRunes(scr, "Checking")
	scr, _ = sendKey(scr, tea.KeyEnter) // accept suggestion
	scr, _ = sendKey(scr, tea.KeyEnter) // account -> category
	scr, _ = sendRunes(scr, "Rent")
	scr, _ = sendKey(scr, tea.KeyEnter) // accept suggestion
	scr, _ = sendKey(scr, tea.KeyEnter) // category -> description
	scr, _ = sendRunes(scr, "Monthly rent")
	scr, _ = sendKey(scr, tea.KeyEnter) // -> amount
	scr, _ = sendRunes(scr, "-850")
	scr, _ = sendKey(scr, tea.KeyEnter) // -> every
	scr, _ = sendRunes(scr, "1")
	scr, _ = sendKey(scr, tea.KeyEnter) // -> unit
	scr, _ = sendRunes(scr, "months")
	scr, _ = sendKey(scr, tea.KeyEnter) // accept suggestion
	scr, _ = sendKey(scr, tea.KeyEnter) // unit -> day of month
	scr, _ = sendRunes(scr, "1")
	scr, _ = sendKey(scr, tea.KeyEnter) // -> start
	scr, _ = sendRunes(scr, "2026-07-01")
	scr, _ = sendKey(scr, tea.KeyEnter) // -> end
	scr, cmd := sendKey(scr, tea.KeyEnter)
	scr = run(t, scr, cmd)

	if !strings.Contains(scr.View(), "Monthly rent") {
		t.Fatalf("expected the recurrence in the list, got:\n%s", scr.View())
	}

	// Edit the description.
	scr, _ = sendRunes(scr, "e")
	if !strings.Contains(scr.View(), "Edit recurrence") {
		t.Fatalf("expected the edit form to open, got:\n%s", scr.View())
	}
	if !strings.Contains(scr.View(), "(read-only)") {
		t.Fatalf("expected account and start to render read-only, got:\n%s", scr.View())
	}
	scr, _ = sendKey(scr, tea.KeyEnter) // close the category suggestion popup
	scr, _ = sendKey(scr, tea.KeyEnter) // category -> description (unchanged)
	for range "Monthly rent" {
		scr, _ = sendKey(scr, tea.KeyBackspace)
	}
	scr, _ = sendRunes(scr, "Rent payment")
	scr, _ = sendKey(scr, tea.KeyEnter) // description -> amount
	scr, _ = sendKey(scr, tea.KeyEnter) // amount -> every
	scr, _ = sendKey(scr, tea.KeyEnter) // every -> unit
	scr, _ = sendKey(scr, tea.KeyEnter) // close the "months" suggestion popup
	scr, _ = sendKey(scr, tea.KeyEnter) // unit -> day of month
	scr, _ = sendKey(scr, tea.KeyEnter) // day of month -> end
	scr, cmd = sendKey(scr, tea.KeyEnter)
	scr = run(t, scr, cmd)
	if !strings.Contains(scr.View(), "Rent payment") {
		t.Fatalf("expected Rent payment after edit, got:\n%s", scr.View())
	}

	// Remove it.
	scr, cmd = sendRunes(scr, "d")
	scr = run(t, scr, cmd)
	if !strings.Contains(scr.View(), "Delete recurrence") {
		t.Fatalf("expected a delete confirmation, got:\n%s", scr.View())
	}
	scr, cmd = sendRunes(scr, "y")
	scr = run(t, scr, cmd)
	if strings.Contains(scr.View(), "Rent payment") {
		t.Fatalf("expected the recurrence removed, got:\n%s", scr.View())
	}
}
