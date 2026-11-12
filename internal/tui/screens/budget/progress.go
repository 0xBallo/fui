package budget

import (
	"fmt"
	"strings"

	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/theme"
)

// barWidth is the number of filled/empty blocks drawn in a progress bar.
const barWidth = 10

// stateLabel turns a BudgetState into the text the spec requires alongside (not instead of)
// color, so a non-color terminal still shows the threshold state.
func stateLabel(s domain.BudgetState) string {
	switch s {
	case domain.BudgetOver:
		return "OVER"
	case domain.BudgetWarning:
		return "WARNING"
	case domain.BudgetOK:
		return "OK"
	default:
		return "OK"
	}
}

// progressBar renders a percent (0..100, clamped) as filled blocks, a percentage and a state
// label, colored by state but never relying on color alone.
func progressBar(th theme.Theme, percent int, state domain.BudgetState) string {
	p := percent
	if p < 0 {
		p = 0
	}
	if p > 100 {
		p = 100
	}
	filled := (p * barWidth) / 100
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	style := th.Positive
	switch state {
	case domain.BudgetWarning:
		style = th.Warning
	case domain.BudgetOver:
		style = th.Negative
	case domain.BudgetOK:
		style = th.Positive
	}
	return fmt.Sprintf("%s %3d%% %s", style.Render(bar), percent, stateLabel(state))
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

// envelopeState mirrors the cap thresholds (80%/100%) for an envelope's progress bar, since
// EnvelopeMonth carries no Budget.Thresholds of its own in the domain engine.
func envelopeState(m domain.EnvelopeMonth) domain.BudgetState {
	if m.Overspend.Sign() > 0 {
		return domain.BudgetOver
	}
	if envelopePercent(m) >= domain.DefaultBudgetThresholds.Warn {
		return domain.BudgetWarning
	}
	return domain.BudgetOK
}
