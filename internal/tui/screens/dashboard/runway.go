package dashboard

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// runwaySection is a placeholder until the runway block is implemented.
type runwaySection struct{}

func (runwaySection) load(sectionParams) tea.Cmd     { return nil }
func (runwaySection) update(tea.Msg) (tea.Cmd, bool) { return nil, false }
func (runwaySection) view(int) string                { return "" }
func (runwaySection) keys() []key.Binding            { return nil }
