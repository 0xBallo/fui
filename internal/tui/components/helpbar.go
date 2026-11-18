package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"

	"github.com/0xballo/fui/internal/tui/theme"
)

// HelpBar renders key hints as one truncated line, or expanded as titled groups.
type HelpBar struct {
	help     help.Model
	th       theme.Theme
	width    int
	expanded bool
}

// NewHelpBar returns a collapsed help bar styled with th.
func NewHelpBar(th theme.Theme) HelpBar {
	m := help.New()
	m.Styles.ShortKey = th.HelpKey
	m.Styles.ShortDesc = th.HelpDesc
	m.Styles.ShortSeparator = th.HelpDesc
	m.Styles.Ellipsis = th.HelpDesc
	m.ShortSeparator = " · "
	return HelpBar{help: m, th: th}
}

// SetWidth sets the width the collapsed line is truncated to.
func (h *HelpBar) SetWidth(w int) {
	h.width = w
	h.help.Width = w
}

// Toggle switches between collapsed and expanded.
func (h *HelpBar) Toggle() { h.expanded = !h.expanded }

// Expanded reports whether the bar is expanded.
func (h HelpBar) Expanded() bool { return h.expanded }

// View renders the bar. Collapsed it is a single line with the screen bindings first, then the
// global ones, without plain navigation keys. Expanded it lists the "Global" and screenTitle
// groups, flowing into side-by-side columns when they exceed maxHeight lines.
func (h HelpBar) View(global, screen []key.Binding, screenTitle string, maxHeight int) string {
	if !h.expanded {
		all := append(append([]key.Binding{}, usable(screen, true)...), usable(global, true)...)
		return h.help.ShortHelpView(all)
	}
	groups := []string{h.group("Global", usable(global, false), maxHeight)}
	if s := usable(screen, false); len(s) > 0 {
		groups = append(groups, h.group(screenTitle, s, maxHeight))
	}
	total := 0
	for _, g := range groups {
		total += lipgloss.Height(g)
	}
	if maxHeight <= 0 || total <= maxHeight {
		return strings.Join(groups, "\n")
	}
	for i := range groups[:len(groups)-1] {
		groups[i] = lipgloss.NewStyle().PaddingRight(4).Render(groups[i])
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, groups...)
}

// group renders a titled block, splitting its entries into columns that fit maxHeight below the title.
func (h HelpBar) group(title string, bs []key.Binding, maxHeight int) string {
	head := h.th.Title.Render(title)
	perCol := len(bs)
	if maxHeight > 0 {
		perCol = min(perCol, max(1, maxHeight-lipgloss.Height(head)))
	}
	var cols []string
	for start := 0; start < len(bs); start += perCol {
		end := min(start+perCol, len(bs))
		keyW := 0
		for _, b := range bs[start:end] {
			keyW = max(keyW, lipgloss.Width(b.Help().Key))
		}
		lines := make([]string, 0, end-start)
		for _, b := range bs[start:end] {
			hb := b.Help()
			lines = append(lines, h.th.HelpKey.Width(keyW).Render(hb.Key)+"  "+h.th.HelpDesc.Render(hb.Desc))
		}
		col := strings.Join(lines, "\n")
		if end < len(bs) {
			col = lipgloss.NewStyle().PaddingRight(4).Render(col)
		}
		cols = append(cols, col)
	}
	return head + "\n" + lipgloss.JoinHorizontal(lipgloss.Top, cols...)
}

// usable drops disabled and help-less bindings, and navigation keys when skipNav is set.
func usable(bs []key.Binding, skipNav bool) []key.Binding {
	out := make([]key.Binding, 0, len(bs))
	for _, b := range bs {
		if b.Help().Key == "" || !b.Enabled() || (skipNav && isNavigation(b)) {
			continue
		}
		out = append(out, b)
	}
	return out
}

// isNavigation reports whether b moves the cursor or backs out rather than running a command.
func isNavigation(b key.Binding) bool {
	for _, k := range b.Keys() {
		switch k {
		case "up", "down", "pgup", "pgdown", "home", "end", "esc":
			return true
		}
	}
	return false
}
