package transactions

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/screens"
)

const (
	// bigTotalsMinHeight is the smallest screen height that can afford the 3-row digits.
	bigTotalsMinHeight = 20
	bigTotalsRows      = 5 // label row, 3 digit rows, blank spacer
	compactTotalsRows  = 2 // one line plus a blank spacer
)

// totalsState is the last totals fetched for the current filter; err marks a failed fetch
// (typically a missing exchange rate) and ok that nothing has been fetched yet.
type totalsState struct {
	ok  bool
	err error
	val app.TxTotals
}

// totalsMsg carries totals for a search text back into Update; it is dropped when the
// text no longer matches the active filter.
type totalsMsg struct {
	text string
	val  app.TxTotals
	err  error
}

// fetchTotals runs TxTotals over req, ignoring paging, and returns the result as a totalsState.
func fetchTotals(ctx context.Context, uc UseCases, req app.ListTxRequest) totalsState {
	t, err := uc.TxTotals(ctx, req)
	return totalsState{ok: true, err: err, val: t}
}

// totalsCmd refreshes the totals after a search changed the filter text.
func (m *Model) totalsCmd() tea.Cmd {
	uc, req := m.uc, m.req
	return func() tea.Msg {
		s := fetchTotals(context.Background(), uc, req)
		return totalsMsg{text: req.Text, val: s.val, err: s.err}
	}
}

func (m *Model) bigTotals() bool {
	if m.height < bigTotalsMinHeight || !m.totals.ok || m.totals.err != nil {
		return false
	}
	return m.width >= bigBlockWidth(m.totalsTexts())*2+2
}

func bigBlockWidth(in, out string) int {
	return max(components.BigTextWidth(in), components.BigTextWidth(out))
}

func (m *Model) totalsTexts() (in, out string) {
	t := m.totals.val
	return "+" + domain.FormatAmount(t.Income, m.loc), domain.FormatAmount(t.Expense, m.loc)
}

// totalsHeight is the number of rows totalsView occupies, so the table can shrink by it.
func (m *Model) totalsHeight() int {
	if m.bigTotals() {
		return bigTotalsRows
	}
	return compactTotalsRows
}

func (m *Model) totalsView() string {
	if !m.totals.ok {
		return "\n"
	}
	if m.totals.err != nil {
		return m.th.Muted.Render("totals n/a") + "\n"
	}
	in, out := m.totalsTexts()
	if m.bigTotals() {
		w := m.width / 2
		left := lipgloss.NewStyle().Width(w).Render(m.th.Muted.Render("IN") + "\n" + components.BigText(in, m.th.Positive))
		right := m.th.Muted.Render("OUT") + "\n" + components.BigText(out, m.th.Negative)
		return lipgloss.JoinHorizontal(lipgloss.Top, left, right) + "\n"
	}
	t := m.totals.val
	line := fmt.Sprintf("IN %s · OUT %s · %d tx", in, strings.Replace(out, "-", "−", 1), t.Count)
	return lipgloss.NewStyle().Bold(true).Render(line) + "\n"
}

// applySearchTotals applies a search result and refreshes the totals for the new filter text.
func (m *Model) applySearchTotals(msg searchResultMsg) (screens.Screen, tea.Cmd) {
	s, cmd := m.applySearchResult(msg)
	if msg.err == nil && msg.seq == m.searchSeq {
		cmd = tea.Batch(cmd, m.totalsCmd())
	}
	return s, cmd
}

func (m *Model) applyTotals(msg totalsMsg) {
	if msg.text != m.req.Text {
		return
	}
	m.totals = totalsState{ok: true, err: msg.err, val: msg.val}
	m.table.SetHeight(max(3, m.height-8-m.totalsHeight()))
}
