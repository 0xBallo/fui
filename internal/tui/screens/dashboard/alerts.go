package dashboard

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// alertsSection is a placeholder until the alerts block is implemented.
type alertsSection struct{}

func (alertsSection) load(sectionParams) tea.Cmd     { return nil }
func (alertsSection) update(tea.Msg) (tea.Cmd, bool) { return nil, false }
func (alertsSection) view(int) string                { return "" }
func (alertsSection) keys() []key.Binding            { return nil }
