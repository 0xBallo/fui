package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/0xballo/fui/internal/tui/theme"
)

// mdFooter is the key hint under the scrolled text.
const mdFooter = "↑/↓ PgUp/PgDn scroll · esc close"

// MarkdownModal shows a scrollable markdown document framed by Modal.
type MarkdownModal struct {
	th       theme.Theme
	markdown string
	vp       viewport.Model
	w, h     int
	open     bool
}

// NewMarkdownModal returns a closed modal for markdown, styled from th.
func NewMarkdownModal(th theme.Theme, markdown string) MarkdownModal {
	m := MarkdownModal{th: th, markdown: markdown, vp: viewport.New(0, 0)}
	m.SetSize(80, 24)
	return m
}

// Open shows the modal from the top of the document.
func (m *MarkdownModal) Open() {
	m.open = true
	m.vp.GotoTop()
}

// Close hides the modal.
func (m *MarkdownModal) Close() { m.open = false }

// IsOpen reports whether the modal is visible.
func (m MarkdownModal) IsOpen() bool { return m.open }

// SetSize sets the area the modal is centered in and re-renders the markdown wrapped to the
// inner width, so the text always fits the frame.
func (m *MarkdownModal) SetSize(w, h int) {
	m.w, m.h = w, h
	innerW := max(w-m.th.Dialog.GetHorizontalFrameSize()-2, 10)
	innerH := max(h-m.th.Dialog.GetVerticalFrameSize()-2, 3)
	m.vp.Width, m.vp.Height = innerW, innerH-1
	m.vp.SetContent(m.render(innerW))
}

// render falls back to the raw markdown when glamour fails.
func (m MarkdownModal) render(width int) string {
	style := "dark"
	if m.th.Name == "latte" {
		style = "light"
	}
	profile := lipgloss.ColorProfile()
	if profile == termenv.Ascii {
		// Dark/light still emit bold/underline sequences without colors; notty emits none.
		style = "notty"
	}
	// The profile makes glamour honor NO_COLOR and downsample on 16/256-color terminals.
	r, err := glamour.NewTermRenderer(glamour.WithStandardStyle(style), glamour.WithWordWrap(width),
		glamour.WithColorProfile(profile))
	if err != nil {
		return m.markdown
	}
	out, err := r.Render(m.markdown)
	if err != nil {
		return m.markdown
	}
	return strings.Trim(out, "\n")
}

// Update scrolls on navigation keys and closes on esc or "i".
func (m MarkdownModal) Update(msg tea.Msg) (MarkdownModal, tea.Cmd) {
	if !m.open {
		return m, nil
	}
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "esc", "i":
			m.open = false
			return m, nil
		case "up", "k":
			m.vp.ScrollUp(1)
		case "down", "j":
			m.vp.ScrollDown(1)
		case "pgup":
			m.vp.PageUp()
		case "pgdown":
			m.vp.PageDown()
		case "home":
			m.vp.GotoTop()
		case "end":
			m.vp.GotoBottom()
		}
	}
	return m, nil
}

// View renders the framed document, or "" when closed.
func (m MarkdownModal) View() string {
	if !m.open {
		return ""
	}
	return Modal(m.th, m.w, m.h, m.vp.View()+"\n"+m.th.Muted.Render(mdFooter))
}
