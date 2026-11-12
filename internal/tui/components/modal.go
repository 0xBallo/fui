package components

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/0xballo/fui/internal/tui/theme"
)

// Modal frames content in the theme's dialog box and centers it in a w×h area filled with a faint
// pattern, so a form or confirmation reads as a layer above the screen it interrupts. lipgloss
// cannot composite layers, so the screen behind is replaced rather than shown dimmed.
func Modal(th theme.Theme, w, h int, content string) string {
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, th.Dialog.Render(content),
		lipgloss.WithWhitespaceChars("╱"), lipgloss.WithWhitespaceForeground(th.Tokens.Surface))
}
