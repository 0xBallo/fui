package budget

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/theme"
)

// barWidth is the number of filled/empty blocks drawn in a progress bar.
const barWidth = 10

// tier maps a percentage to its visual tier icon and style: green below 70, yellow 70-89, orange
// 90-99, red from 100. The tiers are fixed and visual only (the budget's own thresholds drive
// alerts and status elsewhere). The icon keeps the state readable without color; OVER stays as text.
func tier(th theme.Theme, percent int) (string, lipgloss.Style) {
	switch {
	case percent >= 100:
		return "✗ OVER", th.Error
	case percent >= 90:
		return "‼", th.Caution
	case percent >= 70:
		return "!", th.Warning
	default:
		return "✓", th.Positive
	}
}

// progressBar renders a percent (fill clamped to 0..100) as blocks, the percentage and the tier
// icon, colored by tier but never relying on color alone.
func progressBar(th theme.Theme, percent int) string {
	p := min(max(percent, 0), 100)
	filled := (p * barWidth) / 100
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	icon, style := tier(th, percent)
	return fmt.Sprintf("%s %3d%% %s", style.Render(bar), percent, icon)
}

// envelopeAvailable is CarryIn + Assigned + MovesIn - MovesOut, the amount an envelope's Spent is
// measured against for its own progress bar. This is a display-only figure: the domain engine
// tracks Balance and Overspend, not a percentage.
func envelopeAvailable(m domain.EnvelopeMonth) int64 {
	return m.CarryIn.Minor() + m.Assigned.Minor() + m.MovesIn.Minor() - m.MovesOut.Minor()
}

// envelopePercent is Spent as a percentage of envelopeAvailable, floored at 0 and capped at 100
// when nothing is available (and something was spent) or 0 when nothing was spent.
func envelopePercent(m domain.EnvelopeMonth) int {
	avail := envelopeAvailable(m)
	spent := m.Spent.Minor()
	if spent <= 0 {
		return 0
	}
	if avail <= 0 {
		return 100
	}
	return int(spent * 100 / avail)
}
