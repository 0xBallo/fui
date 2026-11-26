// Package transactions provides the register view for inspecting, searching and managing transactions.
package transactions

import "charm.land/bubbles/v2/key"

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
	Split       key.Binding
	Tags        key.Binding
	Categorize  key.Binding
	ClearFilter key.Binding
	PrevChip    key.Binding
	NextChip    key.Binding
	RemoveChip  key.Binding
	// ConfirmPending and SkipPending act on a pending row. ConfirmPending shares "c" with
	// Categorize, which keeps it on every other row, so both stay out of Bindings (no false
	// collision) and are added to the help only when the PendingActions port is wired.
	ConfirmPending key.Binding
	SkipPending    key.Binding
	// PickDate opens the date picker on a focused From/To filter field. It shares ctrl+d with
	// PageDown, which is a list-mode key, so it is left out of Bindings to avoid a false collision.
	PickDate key.Binding
}

// DefaultKeyMap returns the canonical key bindings for transactions.
func DefaultKeyMap() KeyMap {
	km := KeyMap{
		Up:          bind("k/↑", "up", "k", "up"),
		Down:        bind("j/↓", "down", "j", "down"),
		PageUp:      bind("pgup", "page up", "pgup", "ctrl+u"),
		PageDown:    bind("pgdn", "page down", "pgdown", "ctrl+d"),
		Top:         bind("g/home", "top", "g", "home"),
		Bottom:      bind("G/end", "bottom", "G", "end"),
		Add:         bind("a", "add", "a"),
		Edit:        bind("e/enter", "edit", "e", "enter"),
		Delete:      bind("d", "delete", "d"),
		Filter:      bind("f", "filter", "f"),
		Search:      bind("/", "search", "/"),
		Select:      bind("space", "select", "space"),
		Split:       bind("s", "split", "s"),
		Tags:        bind("t", "tags", "t"),
		Categorize:  bind("c", "bulk categorize", "c"),
		ClearFilter: bind("ctrl+r", "reset filter", "ctrl+r"),
		PickDate:    bind("ctrl+d", "pick date", "ctrl+d"),

		ConfirmPending: bind("c", "confirm pending", "c"),
		SkipPending:    bind("S", "skip pending", "S"),
	}
	km.PrevChip, km.NextChip, km.RemoveChip = chipKeyBindings()
	return km
}

func bind(helpKey, helpDesc string, keys ...string) key.Binding {
	return key.NewBinding(key.WithKeys(keys...), key.WithHelp(helpKey, helpDesc))
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
		k.Select, k.Split, k.Tags, k.Categorize, k.ClearFilter,
		k.PrevChip, k.NextChip, k.RemoveChip,
	}
}
