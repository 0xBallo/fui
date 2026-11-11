package components_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/go-cmp/cmp"

	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/theme"
)

func TestAutocomplete(t *testing.T) {
	ac := components.NewAutocomplete("Category:", "select category", theme.Default())
	ac.SetSuggestions([]string{"Food", "Fun", "Fees", "Housing"})

	ac.SetValue("F")
	ac.Focus()

	view := ac.View()
	if !strings.Contains(view, "Category:") {
		t.Errorf("expected view to contain label, got: %s", view)
	}

	// Press tab/down to cycle suggestions
	ac, _ = ac.Update(tea.KeyMsg{Type: tea.KeyTab})
	// Press enter to accept suggestion
	ac, _ = ac.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if diff := cmp.Diff("Fun", ac.Value()); diff != "" {
		t.Errorf("accepted suggestion mismatch (-want +got):\n%s", diff)
	}

	ac.Blur()
	if ac.Focused() {
		t.Errorf("expected autocomplete to be blurred")
	}
}
