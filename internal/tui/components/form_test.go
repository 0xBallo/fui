package components_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/go-cmp/cmp"

	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/theme"
)

func TestForm(t *testing.T) {
	a := components.NewAutocomplete("Account:", "account", theme.Default())
	a.SetSuggestions([]string{"Checking", "Savings"})
	b := components.NewAutocomplete("Amount:", "amount", theme.Default())

	form := components.NewForm(&a, &b)
	form.Focus()

	if !a.Focused() {
		t.Fatalf("expected first field focused after Focus()")
	}
	if form.AtLast() {
		t.Fatalf("expected not at last field with two fields and focus on the first")
	}

	form.Next()
	if a.Focused() {
		t.Errorf("expected first field blurred after Next()")
	}
	if !b.Focused() {
		t.Errorf("expected second field focused after Next()")
	}
	if !form.AtLast() {
		t.Errorf("expected AtLast() on the second field")
	}

	form.Prev()
	if !a.Focused() {
		t.Errorf("expected first field refocused after Prev()")
	}

	// Typing goes to the focused field.
	form.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Che")})
	if diff := cmp.Diff("Che", a.Value()); diff != "" {
		t.Errorf("value mismatch (-want +got):\n%s", diff)
	}
	if !form.FocusedOpen() {
		t.Errorf("expected the suggestion popup open while matches exist")
	}

	view := form.View()
	if view == "" {
		t.Errorf("expected a non-empty rendered view")
	}
}
