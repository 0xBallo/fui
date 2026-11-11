package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/0xballo/fui/internal/tui/theme"
)

// Autocomplete wraps a text input with filtering suggestions for rapid keyboard completion.
type Autocomplete struct {
	label       string
	input       textinput.Model
	suggestions []string
	filtered    []string
	cursor      int
	open        bool
	theme       theme.Theme
}

// NewAutocomplete builds a single-line input paired with an autocompletion popup.
func NewAutocomplete(label, placeholder string, th theme.Theme) Autocomplete {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Prompt = ""

	return Autocomplete{
		label: label,
		input: ti,
		theme: th,
	}
}

// SetSuggestions assigns available completion values and refreshes the filtered subset.
func (a *Autocomplete) SetSuggestions(items []string) {
	a.suggestions = items
	a.filter()
}

// Value returns the current string in the input field.
func (a *Autocomplete) Value() string {
	return a.input.Value()
}

// SetValue populates the field and updates suggestion matching.
func (a *Autocomplete) SetValue(v string) {
	a.input.SetValue(v)
	a.filter()
}

// Focus enables input capture on the text box.
func (a *Autocomplete) Focus() tea.Cmd {
	a.open = true
	a.filter()
	return a.input.Focus()
}

// Blur dismisses focus and hides candidate suggestions.
func (a *Autocomplete) Blur() {
	a.open = false
	a.input.Blur()
}

// Focused indicates whether the field currently owns keyboard focus.
func (a *Autocomplete) Focused() bool {
	return a.input.Focused()
}

// DropdownOpen reports whether the suggestion popup is currently shown, so a containing form
// can let the field consume the next key (e.g. enter to pick a suggestion) before acting itself.
func (a *Autocomplete) DropdownOpen() bool {
	return a.open && len(a.filtered) > 0
}

// Update processes key events, navigating suggestion popups when open.
func (a *Autocomplete) Update(msg tea.Msg) (Autocomplete, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok && a.input.Focused() {
		switch keyMsg.String() {
		case "down", "tab":
			if len(a.filtered) > 0 {
				a.cursor = (a.cursor + 1) % len(a.filtered)
				return *a, nil
			}
		case "up":
			if len(a.filtered) > 0 {
				a.cursor = (a.cursor - 1 + len(a.filtered)) % len(a.filtered)
				return *a, nil
			}
		case "enter":
			if a.open && len(a.filtered) > 0 && a.cursor < len(a.filtered) {
				a.input.SetValue(a.filtered[a.cursor])
				a.open = false
				return *a, nil
			}
		case "esc":
			if a.open {
				a.open = false
				return *a, nil
			}
		}
	}

	var cmd tea.Cmd
	a.input, cmd = a.input.Update(msg)
	a.filter()
	return *a, cmd
}

func (a *Autocomplete) filter() {
	val := strings.ToLower(strings.TrimSpace(a.input.Value()))
	if val == "" {
		a.filtered = a.suggestions
		a.cursor = 0
		return
	}
	var res []string
	for _, item := range a.suggestions {
		if strings.Contains(strings.ToLower(item), val) {
			res = append(res, item)
		}
	}
	a.filtered = res
	if a.cursor >= len(a.filtered) {
		a.cursor = 0
	}
}

// View outputs the field label, the input text, and any visible candidate box.
func (a *Autocomplete) View() string {
	var b strings.Builder
	if a.label != "" {
		b.WriteString(a.theme.Header.Render(a.label))
		b.WriteString(" ")
	}
	b.WriteString(a.input.View())

	if a.open && a.input.Focused() && len(a.filtered) > 0 {
		b.WriteByte('\n')
		b.WriteString(a.renderDropdown())
	}
	return b.String()
}

func (a *Autocomplete) renderDropdown() string {
	var lines []string
	limit := min(5, len(a.filtered))
	for i := 0; i < limit; i++ {
		text := "  " + a.filtered[i]
		if i == a.cursor {
			lines = append(lines, a.theme.Selected.Render("> "+a.filtered[i]))
		} else {
			lines = append(lines, a.theme.Muted.Render(text))
		}
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(a.theme.Border.GetBorderTopForeground()).
		Render(strings.Join(lines, "\n"))
	return box
}
