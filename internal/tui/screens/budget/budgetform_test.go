package budget_test

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	scr "github.com/0xballo/fui/internal/tui/screens"
)

// submit presses enter and runs the resulting command; it returns the screen and the error the
// screen would have sent to the status bar, if any.
func submit(t *testing.T, s scr.Screen) (scr.Screen, error) {
	t.Helper()
	s, cmd := sendKey(s, tea.KeyEnter)
	for cmd != nil {
		msg := cmd()
		if em, ok := msg.(scr.ErrorMsg); ok {
			return s, em.Err
		}
		s, cmd = s.Update(msg)
	}
	return s, nil
}

func typeInto(s scr.Screen, text string) scr.Screen {
	s, _ = sendRunes(s, text)
	return s
}

func next(s scr.Screen) scr.Screen {
	s, _ = sendKey(s, tea.KeyEnter)
	return s
}

func plain(s scr.Screen) string { return ansi.Strip(s.View()) }

func TestBudgetNewEditFlow(t *testing.T) {
	h := newHarness(t, domain.BudgetEnvelope)
	h.fx.Category(t, app.AddCategoryRequest{Name: "Groceries", Kind: "expense"})
	h.fx.Category(t, app.AddCategoryRequest{Name: "Transport", Kind: "expense"})
	h.fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "1000", OpenedOn: "2026-01-01"})

	s := newFixture(t, h)
	s = run(t, s, s.Init())
	if v := plain(s); !strings.Contains(v, "n to create a budget") {
		t.Fatalf("empty state lacks the hint:\n%s", v)
	}

	s, cmd := sendRunes(s, "n")
	s = run(t, s, cmd) // loads the suggestions
	if !s.InputActive() || !strings.Contains(plain(s), "New budget") {
		t.Fatalf("n did not open the new form:\n%s", plain(s))
	}
	s = typeInto(s, "Food")
	s = next(s)
	s = typeInto(s, "gro") // picker: enter picks the suggestion and keeps the field open
	s = next(s)
	if v := plain(s); !strings.Contains(v, "Groceries") {
		t.Fatalf("picked category not shown as chip:\n%s", v)
	}
	s = next(s) // leave categories on empty input
	s = next(s) // tags: skip
	s = typeInto(s, "ye")
	s = next(s) // pick "year"
	s = next(s) // advance to amount
	s = typeInto(s, "600")
	s = next(s) // carry
	s = next(s) // warn
	s = next(s) // over
	s, err := submit(t, s)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if s.InputActive() {
		t.Fatalf("form should close after a successful save:\n%s", plain(s))
	}
	if v := plain(s); !strings.Contains(v, "Food [year]") {
		t.Fatalf("new row not shown:\n%s", v)
	}

	// s overrides this month only; the budget's default amount is untouched.
	s, _ = sendRunes(s, "s")
	s = typeInto(s, "50")
	s, err = submit(t, s)
	if err != nil {
		t.Fatalf("set: %v", err)
	}
	s, _ = sendRunes(s, "a")
	s = typeInto(s, "200")
	if s, err = submit(t, s); err != nil {
		t.Fatalf("assign: %v", err)
	}
	// a second envelope to move into
	h.budget(t, "Transport", "Transport", "0")
	s = run(t, s, s.Init())
	s, _ = sendRunes(s, "m")
	s = typeInto(s, "Transport")
	s = next(s)
	s = next(s)
	s = typeInto(s, "10")
	if _, err = submit(t, s); err != nil {
		t.Fatalf("move: %v", err)
	}
	var got []string
	err = h.fx.Store.Read(context.Background(), func(r app.Repos) error {
		from, _ := domain.NewDate(2026, 6, 1)
		to, _ := domain.NewDate(2026, 6, 30)
		moves, err := r.Budgets().Moves(context.Background(), from, to)
		for _, mv := range moves {
			got = append(got, mv.Date.String())
		}
		return err
	})
	if err != nil || len(got) != 2 {
		t.Fatalf("moves in June = %v (%v), want 2", got, err)
	}
	for _, d := range got {
		if !strings.HasPrefix(d, "2026-06") {
			t.Errorf("move dated %s outside the month on screen", d)
		}
	}

	// edit: change thresholds, the row detail reflects them.
	s = run(t, s, s.Init())
	s, _ = sendRunes(s, "e")
	if v := plain(s); !strings.Contains(v, "Edit budget") || !strings.Contains(v, "Name: Food") {
		t.Fatalf("edit form not prefilled:\n%s", v)
	}
	for range 7 { // categories, tags, period (+pick), amount, carry (+pick)
		s = next(s)
	}
	s, _ = s.Update(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	s = typeInto(s, "60")
	s = next(s)
	s, _ = s.Update(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	s = typeInto(s, "90")
	if s, err = submit(t, s); err != nil {
		t.Fatalf("edit: %v", err)
	}
	if v := plain(s); !strings.Contains(v, "Food: year, thresholds 60/90") {
		t.Fatalf("edit not reflected:\n%s", v)
	}
}

func TestBudgetFormKeepsOpenOnError(t *testing.T) {
	h := newHarness(t, domain.BudgetCap)
	h.fx.Category(t, app.AddCategoryRequest{Name: "Groceries", Kind: "expense"})
	s := newFixture(t, h)
	s = run(t, s, s.Init())

	s, cmd := sendRunes(s, "n")
	s = run(t, s, cmd)
	s = typeInto(s, "Food")
	s = next(s)
	s = typeInto(s, "Nope")
	for range 6 {
		s = next(s)
	}
	s, err := submit(t, s)
	if err == nil {
		t.Fatal("unknown category should fail")
	}
	if !s.InputActive() || !strings.Contains(plain(s), "Food") {
		t.Fatalf("form lost its input after an error:\n%s", plain(s))
	}

	// bad thresholds: warn >= over is rejected by the app, a non-number never reaches it.
	for _, bad := range []string{"150", "abc"} {
		s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
		s, _ = s.Update(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
		s = typeInto(s, bad)
		s = next(s)
		if s, err = submit(t, s); err == nil {
			t.Fatalf("warn %q should fail", bad)
		}
		if !s.InputActive() {
			t.Fatal("form closed on a validation error")
		}
	}
}

// TestEditFormPrefillsTargets checks that the edit form shows the budget's current categories as
// chips, resolved from their ids.
func TestEditFormPrefillsTargets(t *testing.T) {
	h := capHarness(t)
	s := newFixture(t, h)
	s = run(t, s, s.Init())
	s, cmd := sendRunes(s, "e")
	s = run(t, s, cmd)
	if v := plain(s); !strings.Contains(v, "Edit budget") || strings.Count(v, "Groceries") < 2 {
		t.Fatalf("edit form lacks the current target chip (name + chip):\n%s", v)
	}
}
