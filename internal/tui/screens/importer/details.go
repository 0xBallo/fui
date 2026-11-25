package importer

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/screens"
)

type lineKind int

const (
	lineHeader lineKind = iota
	lineItem
	lineWarning
	lineError
)

// detailLine is one line of the preview's details view; transfer is the index of the transfer
// proposal the line stands for, or -1.
type detailLine struct {
	text     string
	kind     lineKind
	transfer int
}

// detailSummary is the one-line count of what the details view holds.
func (m *Model) detailSummary() string {
	p := m.plan
	accepted := 0
	for _, t := range p.Transfers {
		if t.Accept {
			accepted++
		}
	}
	s := fmt.Sprintf("warnings=%d transfers=%d/%d mismatches=%d modules=%d",
		len(p.Warnings), accepted, len(p.Transfers), len(p.BalanceMismatches), len(p.ModuleRecords))
	if n := len(m.pendingModules()); n > 0 {
		s += fmt.Sprintf(" (enabling %d)", n)
	}
	return s
}

// detailBuilder accumulates the lines of the details view.
type detailBuilder struct{ lines []detailLine }

// section opens a titled section and reports whether n > 0, so empty ones are omitted.
func (b *detailBuilder) section(title string, n int) bool {
	if n == 0 {
		return false
	}
	b.lines = append(b.lines, detailLine{text: fmt.Sprintf("%s (%d)", title, n), kind: lineHeader, transfer: -1})
	return true
}

func (b *detailBuilder) add(kind lineKind, format string, args ...any) {
	b.lines = append(b.lines, detailLine{text: fmt.Sprintf(format, args...), kind: kind, transfer: -1})
}

// buildDetails lists, section by section, everything the plan reports beyond the rows: errors
// and balance discrepancies (with their source line), warnings, transfer proposals, balance
// mismatches with the reconcile command that fixes them, the records of disabled modules and
// the categories, tags and payees the import would create.
func (m *Model) buildDetails() []detailLine {
	var b detailBuilder
	m.problemSections(&b)
	m.transferSections(&b)
	m.newEntitySections(&b)
	return b.lines
}

func (m *Model) problemSections(b *detailBuilder) {
	p := m.plan
	if b.section("Errors", len(p.Errors)) {
		for _, e := range p.Errors {
			b.add(lineError, "line %d: %s", e.Line, e.Message)
		}
	}
	if b.section("Balance discrepancies", len(p.Discrepancies)) {
		for _, d := range p.Discrepancies {
			b.add(lineError, "line %d %s %s: expected %s, got %s", d.Line, d.AccountRef, d.Date.Format(m.loc),
				m.minor(d.Expected), m.minor(d.Actual))
		}
	}
	if b.section("Warnings", len(p.Warnings)) {
		for _, w := range p.Warnings {
			b.add(lineWarning, "line %d: %s", w.Line, w.Message)
		}
	}
}

func (m *Model) transferSections(b *detailBuilder) {
	p := m.plan
	if b.section("Transfer proposals", len(p.Transfers)) {
		for i, t := range p.Transfers {
			b.lines = append(b.lines, detailLine{text: m.transferText(t), kind: lineItem, transfer: i})
		}
	}
	if b.section("Balance mismatches", len(p.BalanceMismatches)) {
		for _, bm := range p.BalanceMismatches {
			b.add(lineWarning, "%s %s: statement %s, computed %s", bm.AccountRef, bm.Date.Format(m.loc),
				m.minor(bm.Expected), m.minor(bm.Computed))
			if bm.Suggest != "" {
				b.add(lineItem, "  fix: %s", bm.Suggest)
			}
		}
	}
	if b.section("Disabled-module records", len(p.ModuleRecords)) {
		for _, r := range p.ModuleRecords {
			b.add(lineItem, "%s: %d record(s) [%s] lines %s", r.Module, r.Count, strings.Join(r.Kinds, ","), lineList(r.Lines))
		}
	}
}

func (m *Model) newEntitySections(b *detailBuilder) {
	p := m.plan
	if b.section("New categories", len(p.NewCategories)) {
		for _, c := range p.NewCategories {
			b.add(lineItem, "%s", c.Name)
		}
	}
	if b.section("New tags", len(p.NewTags)) {
		for _, t := range p.NewTags {
			b.add(lineItem, "%s", t.Name)
		}
	}
	if b.section("New payees", len(p.NewPayees)) {
		for _, y := range p.NewPayees {
			b.add(lineItem, "%s", y.Name)
		}
	}
}

// lineList renders up to 5 line numbers, with a trailing count of the rest.
func lineList(lines []int) string {
	limit := min(5, len(lines))
	parts := make([]string, limit, limit+1)
	for i := range limit {
		parts[i] = strconv.Itoa(lines[i])
	}
	if len(lines) > limit {
		parts = append(parts, fmt.Sprintf("+%d", len(lines)-limit))
	}
	return strings.Join(parts, ",")
}

func (m *Model) transferText(t importing.TransferProposal) string {
	mark := "[ ]"
	if t.Accept {
		mark = "[x]"
	}
	partner := fmt.Sprintf("existing tx #%v", t.Existing)
	if t.Other >= 0 && t.Other < len(m.plan.Rows) {
		partner = fmt.Sprintf("line %d", m.plan.Rows[t.Other].Line)
	}
	own := strconv.Itoa(t.Row)
	if t.Row >= 0 && t.Row < len(m.plan.Rows) {
		own = fmt.Sprintf("line %d %s", m.plan.Rows[t.Row].Line, m.plan.Rows[t.Row].Tx.Description)
	}
	return fmt.Sprintf("%s %s <-> %s (%s)", mark, own, partner, t.Reason)
}

func (m *Model) handleDetailsKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch {
	case key.Matches(msg, m.km.Up):
		m.detailCursor = max(0, m.detailCursor-1)
	case key.Matches(msg, m.km.Down):
		m.detailCursor = min(len(m.detailLines)-1, m.detailCursor+1)
	case key.Matches(msg, m.km.Transfer):
		m.toggleTransfer()
	case key.Matches(msg, m.km.Confirm):
		return m.openConfirm()
	case key.Matches(msg, m.km.Back):
		m.details = false
	}
	return m, nil
}

// toggleTransfer flips the acceptance of the transfer proposal under the details cursor.
func (m *Model) toggleTransfer() {
	if m.detailCursor < 0 || m.detailCursor >= len(m.detailLines) {
		return
	}
	i := m.detailLines[m.detailCursor].transfer
	if i < 0 {
		return
	}
	if err := m.plan.SetAcceptTransfer(i, !m.plan.Transfers[i].Accept); err != nil {
		return
	}
	m.detailLines = m.buildDetails()
}

func (m *Model) viewDetails() string {
	if len(m.detailLines) == 0 {
		return m.th.Muted.Render("Nothing to report beyond the rows.")
	}
	height := max(3, m.height-12)
	start := 0
	if m.detailCursor >= height {
		start = m.detailCursor - height + 1
	}
	end := min(len(m.detailLines), start+height)
	lines := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		l := m.detailLines[i]
		prefix := "  "
		if i == m.detailCursor {
			prefix = "> "
		}
		style := m.th.Base
		switch l.kind {
		case lineHeader:
			style = m.th.Header
		case lineWarning:
			style = m.th.Warning
		case lineError:
			style = m.th.Error
		case lineItem:
		}
		lines = append(lines, lipgloss.NewStyle().Render(prefix)+style.Render(l.text))
	}
	return strings.Join(lines, "\n")
}

// minor formats an amount of the plan's currency; stream-level records carry no currency, so
// the one of the first row is used, falling back to bare minor units for a plan without rows.
func (m *Model) minor(v int64) string {
	if len(m.plan.Rows) > 0 {
		if money, err := domain.NewMoney(v, m.plan.Rows[0].Tx.Amount.Currency()); err == nil {
			return domain.FormatAmount(money, m.loc)
		}
	}
	return strconv.FormatInt(v, 10)
}
