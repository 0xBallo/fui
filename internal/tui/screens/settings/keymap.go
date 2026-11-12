package settings

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines the keyboard shortcuts of the settings screen.
type KeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Section key.Binding
	Toggle  key.Binding
	Cancel  key.Binding
}

// DefaultKeyMap returns the canonical key bindings for settings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up:      key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "up")),
		Down:    key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "down")),
		Section: key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch section")),
		Toggle:  key.NewBinding(key.WithKeys("enter", " "), key.WithHelp("enter/space", "enable, disable, or apply")),
		Cancel:  key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "restore saved theme")),
	}
}

// Bindings returns all keys as a slice for help generation and collision checks.
func (k KeyMap) Bindings() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Section, k.Toggle, k.Cancel}
}
