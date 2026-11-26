package importer

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines the keyboard shortcuts of the import screen outside its text forms (the source
// and mapping forms handle their own keys, since every key there feeds a text field).
type KeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Toggle  key.Binding
	Confirm key.Binding
	Back    key.Binding
	New     key.Binding
	// Details switches the preview between the row table and the details sections.
	Details key.Binding
	// Transfer accepts or rejects the transfer proposal under the details cursor.
	Transfer key.Binding
	// Modules opens the dialog to enable or skip the records of disabled modules.
	Modules key.Binding
	// MapCSV opens the CSV column mapper from the source form.
	MapCSV key.Binding
}

// DefaultKeyMap returns the canonical key bindings for the import screen.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up:       key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "up")),
		Down:     key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "down")),
		Toggle:   key.NewBinding(key.WithKeys(" ", "i"), key.WithHelp("space/i", "include/exclude")),
		Confirm:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
		Back:     key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		New:      key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new import")),
		Details:  key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "rows/details")),
		Transfer: key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "accept transfer")),
		Modules:  key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "modules")),
		MapCSV:   key.NewBinding(key.WithKeys("ctrl+p"), key.WithHelp("ctrl+p", "map csv columns")),
	}
}

// Bindings returns all keys as a slice for help generation and collision checks.
func (k KeyMap) Bindings() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Toggle, k.Confirm, k.Back, k.New, k.Details, k.Transfer, k.Modules, k.MapCSV}
}
