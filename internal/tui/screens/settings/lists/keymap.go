package lists

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines the keyboard shortcuts shared by every CRUD list sub-view: browsing, adding,
// editing and removing an entry, plus Test (rules only, ignored elsewhere).
type KeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Add    key.Binding
	Edit   key.Binding
	Remove key.Binding
	Test   key.Binding
	Back   key.Binding
}

// DefaultKeyMap returns the canonical key bindings for the CRUD list sub-views.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up:     key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "up")),
		Down:   key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "down")),
		Add:    key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add")),
		Edit:   key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
		Remove: key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
		Test:   key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "test")),
		Back:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	}
}

// Bindings returns every key but Test, for sub-views without a test mode.
func (k KeyMap) Bindings() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Add, k.Edit, k.Remove, k.Back}
}

// BindingsWithTest returns every key including Test, for the rules sub-view.
func (k KeyMap) BindingsWithTest() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Add, k.Edit, k.Remove, k.Test, k.Back}
}
