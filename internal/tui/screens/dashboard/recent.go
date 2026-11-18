package dashboard

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// recentSection is a placeholder until the recent block is implemented.
type recentSection struct{}

func (recentSection) load(sectionParams) tea.Cmd     { return nil }
func (recentSection) update(tea.Msg) (tea.Cmd, bool) { return nil, false }
func (recentSection) view(int) string                { return "" }
func (recentSection) keys() []key.Binding            { return nil }
