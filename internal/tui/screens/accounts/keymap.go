package accounts

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines the keyboard shortcuts of the accounts screen.
type KeyMap struct {
	Up        key.Binding
	Down      key.Binding
	Open      key.Binding
	Back      key.Binding
	Add       key.Binding
	Archive   key.Binding
	Reconcile key.Binding
}

// DefaultKeyMap returns the canonical key bindings for accounts.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up:        key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "up")),
		Down:      key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "down")),
		Open:      key.NewBinding(key.WithKeys("e", "enter"), key.WithHelp("e/enter", "movements")),
		Back:      key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		Add:       key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add")),
		Archive:   key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "archive")),
		Reconcile: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reconcile")),
	}
}

// Bindings returns all keys as a slice for help generation and collision checks.
func (k KeyMap) Bindings() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Open, k.Back, k.Add, k.Archive, k.Reconcile}
}
