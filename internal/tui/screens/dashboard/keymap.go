package dashboard

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines the keyboard shortcuts of the dashboard screen.
type KeyMap struct {
	Add, Cycle, Prev, Next, Up, Down, PageUp, PageDown key.Binding
}

// DefaultKeyMap returns the canonical key bindings for the dashboard.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Add:      key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "quick add")),
		Cycle:    key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "next cycle")),
		Prev:     key.NewBinding(key.WithKeys("h", "left"), key.WithHelp("h/←", "previous period")),
		Next:     key.NewBinding(key.WithKeys("l", "right"), key.WithHelp("l/→", "next period")),
		Up:       key.NewBinding(key.WithKeys("up")),
		Down:     key.NewBinding(key.WithKeys("down")),
		PageUp:   key.NewBinding(key.WithKeys("pgup")),
		PageDown: key.NewBinding(key.WithKeys("pgdown")),
	}
}

// Bindings returns all keys as a slice for help generation and collision checks.
func (k KeyMap) Bindings() []key.Binding {
	return []key.Binding{k.Add, k.Cycle, k.Prev, k.Next}
}
