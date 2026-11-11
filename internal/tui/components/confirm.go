package components

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/0xballo/fui/internal/tui/theme"
)

// ConfirmDialog presents an explicit yes/no prompt before destructive actions.
type ConfirmDialog struct {
	title   string
	prompt  string
	details []string
	visible bool
	theme   theme.Theme
}

// NewConfirmDialog initializes an inactive confirmation modal.
func NewConfirmDialog(th theme.Theme) ConfirmDialog {
	return ConfirmDialog{theme: th}
}

// Show activates the dialog with a title, a question, and optional contextual details.
func (c *ConfirmDialog) Show(title, prompt string, details ...string) {
	c.title = title
	c.prompt = prompt
	c.details = details
	c.visible = true
}

// Hide closes the confirmation dialog.
func (c *ConfirmDialog) Hide() {
	c.visible = false
	c.details = nil
}

// Visible reports whether the dialog is awaiting user interaction.
func (c *ConfirmDialog) Visible() bool {
	return c.visible
}

// HandleKey inspects a key message, returning whether the user confirmed or cancelled.
func (c *ConfirmDialog) HandleKey(msg tea.KeyMsg) (confirmed bool, cancelled bool) {
	if !c.visible {
		return false, false
	}
	switch msg.String() {
	case "y", "Y", "enter":
		c.Hide()
		return true, false
	case "n", "N", "esc", "q":
		c.Hide()
		return false, true
	}
	return false, false
}

// View formats the modal box with question and shortcut prompts.
func (c *ConfirmDialog) View() string {
	if !c.visible {
		return ""
	}
	var lines []string
	if c.title != "" {
		lines = append(lines, c.theme.Title.Render(c.title))
	}
	lines = append(lines, c.theme.Base.Render(c.prompt))
	for _, d := range c.details {
		lines = append(lines, c.theme.Muted.Render(d))
	}
	lines = append(lines, "")
	actions := c.theme.HelpKey.Render("[y]") + " Yes   " + c.theme.HelpKey.Render("[n]") + " No"
	lines = append(lines, actions)

	body := strings.Join(lines, "\n")
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(c.theme.Warning.GetForeground()).
		Padding(1, 2).
		Render(body)
}
