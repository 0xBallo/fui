package dashboard

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// flowSection is a placeholder until the flow block is implemented.
type flowSection struct{}

func (flowSection) load(sectionParams) tea.Cmd     { return nil }
func (flowSection) update(tea.Msg) (tea.Cmd, bool) { return nil, false }
func (flowSection) view(int) string                { return "" }
func (flowSection) keys() []key.Binding            { return nil }
