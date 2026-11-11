package components

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Form sequences a set of Autocomplete fields, moving focus forward and backward and tracking
// which one is active. It carries no domain knowledge: screens decide field labels, suggestions
// and what to do with the values.
type Form struct {
	fields []*Autocomplete
	focus  int
}

// NewForm builds a form over fields, in tab order.
func NewForm(fields ...*Autocomplete) Form {
	return Form{fields: fields}
}

// Focus activates the first field.
func (f *Form) Focus() tea.Cmd {
	if len(f.fields) == 0 {
		return nil
	}
	f.focus = 0
	return f.fields[f.focus].Focus()
}

// Next blurs the active field and focuses the following one, wrapping at the end.
func (f *Form) Next() tea.Cmd {
	return f.move(1)
}

// Prev blurs the active field and focuses the previous one, wrapping at the start.
func (f *Form) Prev() tea.Cmd {
	return f.move(-1)
}

func (f *Form) move(delta int) tea.Cmd {
	if len(f.fields) == 0 {
		return nil
	}
	f.fields[f.focus].Blur()
	f.focus = (f.focus + delta + len(f.fields)) % len(f.fields)
	return f.fields[f.focus].Focus()
}

// AtLast reports whether the last field is active, so the screen can submit on enter instead of
// advancing.
func (f *Form) AtLast() bool {
	return len(f.fields) == 0 || f.focus == len(f.fields)-1
}

// FocusedOpen reports whether the active field has an open suggestion popup with typed text to
// match: the screen should forward the key to it (e.g. enter picks the suggestion) rather than
// treat it as a form action. An empty, untouched field also starts with its dropdown open (every
// suggestion matches the empty string), but that must not block Enter from skipping an optional
// field, so it is excluded here.
func (f *Form) FocusedOpen() bool {
	return len(f.fields) > 0 && f.fields[f.focus].DropdownOpen() && f.fields[f.focus].Value() != ""
}

// Update forwards msg to the active field.
func (f *Form) Update(msg tea.Msg) tea.Cmd {
	if len(f.fields) == 0 {
		return nil
	}
	updated, cmd := f.fields[f.focus].Update(msg)
	*f.fields[f.focus] = updated
	return cmd
}

// View renders every field, one per line.
func (f *Form) View() string {
	lines := make([]string, len(f.fields))
	for i, field := range f.fields {
		lines[i] = field.View()
	}
	return strings.Join(lines, "\n")
}
