package dashboard

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/domain"
)

// sectionParams is what every dashboard section needs to load for the selected cycle.
type sectionParams struct {
	Period        domain.Period
	AsOf, Today   domain.Date
	Base          domain.Currency
	MonthStartDay int
}

// section is one block of the dashboard body. Sections own their messages: update reports
// whether msg was consumed so the model stops routing it.
type section interface {
	load(p sectionParams) tea.Cmd       // nil when disabled
	update(msg tea.Msg) (tea.Cmd, bool) // true = consumed (own msg types only, plus keys when focused)
	view(width int) string              // "" = hidden
	keys() []key.Binding
}
