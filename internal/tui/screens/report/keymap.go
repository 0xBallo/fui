package report

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines the keyboard shortcuts of the report screen.
type KeyMap struct {
	Up       key.Binding
	Down     key.Binding
	PrevKind key.Binding
	NextKind key.Binding
	Refresh  key.Binding
	Params   key.Binding
	Save     key.Binding
	Saved    key.Binding
	RunSaved key.Binding
	DelSaved key.Binding
	Export   key.Binding
	Back     key.Binding
}

// DefaultKeyMap returns the canonical key bindings for the report screen.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up:       key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "up")),
		Down:     key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "down")),
		PrevKind: key.NewBinding(key.WithKeys("h", "left"), key.WithHelp("h/←", "previous report")),
		NextKind: key.NewBinding(key.WithKeys("l", "right"), key.WithHelp("l/→", "next report")),
		Refresh:  key.NewBinding(key.WithKeys("r", "enter"), key.WithHelp("r/enter", "run")),
		Params:   key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "change period/params")),
		Save:     key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "save report")),
		Saved:    key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "saved reports")),
		RunSaved: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "run selected")),
		DelSaved: key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete selected")),
		Export:   key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "export transactions")),
		Back:     key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	}
}

// Bindings returns all keys as a slice for help generation and collision checks.
func (k KeyMap) Bindings() []key.Binding {
	return []key.Binding{
		k.Up, k.Down, k.PrevKind, k.NextKind, k.Refresh, k.Params,
		k.Save, k.Saved, k.RunSaved, k.DelSaved, k.Export, k.Back,
	}
}
