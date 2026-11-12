package budget

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines the keyboard shortcuts of the budget screen.
type KeyMap struct {
	Up        key.Binding
	Down      key.Binding
	PrevMonth key.Binding
	NextMonth key.Binding
	SetAmount key.Binding
	Assign    key.Binding
	Move      key.Binding
	CopyPrev  key.Binding
	Back      key.Binding
}

// DefaultKeyMap returns the canonical key bindings for the budget screen.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up:        key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "up")),
		Down:      key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "down")),
		PrevMonth: key.NewBinding(key.WithKeys("h", "left"), key.WithHelp("h/←", "previous month")),
		NextMonth: key.NewBinding(key.WithKeys("l", "right"), key.WithHelp("l/→", "next month")),
		SetAmount: key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "set amount")),
		Assign:    key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "assign from pool")),
		Move:      key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "move to envelope")),
		CopyPrev:  key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "copy previous month")),
		Back:      key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	}
}

// Bindings returns all keys as a slice for help generation and collision checks.
func (k KeyMap) Bindings() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.PrevMonth, k.NextMonth, k.SetAmount, k.Assign, k.Move, k.CopyPrev, k.Back}
}
