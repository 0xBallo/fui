// Package transactions provides the register view for inspecting, searching and managing transactions.
package transactions

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines the keyboard shortcuts for the transactions screen.
type KeyMap struct {
	Up          key.Binding
	Down        key.Binding
	PageUp      key.Binding
	PageDown    key.Binding
	Top         key.Binding
	Bottom      key.Binding
	Add         key.Binding
	Edit        key.Binding
	Delete      key.Binding
	Filter      key.Binding
	Search      key.Binding
	Select      key.Binding
	Categorize  key.Binding
	ClearFilter key.Binding
	PrevChip    key.Binding
	NextChip    key.Binding
	RemoveChip  key.Binding
}

// DefaultKeyMap returns the canonical key bindings for transactions.
func DefaultKeyMap() KeyMap {
	km := KeyMap{
		Up: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("k/↑", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("j/↓", "down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "ctrl+u"),
			key.WithHelp("pgup", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "ctrl+d"),
			key.WithHelp("pgdn", "page down"),
		),
		Top: key.NewBinding(
			key.WithKeys("g", "home"),
			key.WithHelp("g/home", "top"),
		),
		Bottom: key.NewBinding(
			key.WithKeys("G", "end"),
			key.WithHelp("G/end", "bottom"),
		),
		Add: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "add"),
		),
		Edit: key.NewBinding(
			key.WithKeys("e", "enter"),
			key.WithHelp("e/enter", "edit"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete"),
		),
		Filter: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "filter"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		Select: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "select"),
		),
		Categorize: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "bulk categorize"),
		),
		ClearFilter: key.NewBinding(
			key.WithKeys("ctrl+r"),
			key.WithHelp("ctrl+r", "reset filter"),
		),
	}
	km.PrevChip, km.NextChip, km.RemoveChip = chipKeyBindings()
	return km
}

// chipKeyBindings returns the bindings that move focus between filter chips and remove the
// focused one, split out of DefaultKeyMap to keep it short.
func chipKeyBindings() (prev, next, remove key.Binding) {
	prev = key.NewBinding(
		key.WithKeys("left"),
		key.WithHelp("←", "prev filter chip"),
	)
	next = key.NewBinding(
		key.WithKeys("right"),
		key.WithHelp("→", "next filter chip"),
	)
	remove = key.NewBinding(
		key.WithKeys("backspace", "x"),
		key.WithHelp("backspace/x", "remove filter chip"),
	)
	return prev, next, remove
}

// Bindings returns all keys as a slice for help generation and collision checks.
func (k KeyMap) Bindings() []key.Binding {
	return []key.Binding{
		k.Up, k.Down, k.PageUp, k.PageDown, k.Top, k.Bottom,
		k.Add, k.Edit, k.Delete, k.Filter, k.Search,
		k.Select, k.Categorize, k.ClearFilter,
		k.PrevChip, k.NextChip, k.RemoveChip,
	}
}
