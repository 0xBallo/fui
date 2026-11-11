package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines the global keyboard shortcuts handled by Root itself, outside any screen.
type KeyMap struct {
	Dashboard    key.Binding
	Transactions key.Binding
	Accounts     key.Binding
	Help         key.Binding
	Quit         key.Binding
}

// DefaultKeyMap returns the canonical global key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Dashboard:    key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "dashboard")),
		Transactions: key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "transactions")),
		Accounts:     key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "accounts")),
		Help:         key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:         key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
	}
}

// Bindings returns all keys as a slice for help generation and collision checks.
func (k KeyMap) Bindings() []key.Binding {
	return []key.Binding{k.Dashboard, k.Transactions, k.Accounts, k.Help, k.Quit}
}
