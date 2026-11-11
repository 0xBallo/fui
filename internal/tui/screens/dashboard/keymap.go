package dashboard

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines the keyboard shortcuts of the dashboard screen.
type KeyMap struct {
	Add key.Binding
}

// DefaultKeyMap returns the canonical key bindings for the dashboard.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Add: key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "quick add")),
	}
}

// Bindings returns all keys as a slice for help generation and collision checks.
func (k KeyMap) Bindings() []key.Binding {
	return []key.Binding{k.Add}
}
