package report

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/theme"
)

// minBarScreen is the screen height below which the bar chart is hidden so the table keeps room.
const minBarScreen = 18

// amountFormatter renders an amount for a bar label.
type amountFormatter func(reporting.Amount) string

// barRowCap is the most bar rows drawn on a screen of height h.
func barRowCap(h int) int {
	if h < minBarScreen {
		return 0
	}
	return h / 3
}

// barItems maps a loaded result to its horizontal bars; it returns nil for kinds without bars.
func barItems(th theme.Theme, fmtAmt amountFormatter, data any) []components.BarItem {
	switch d := data.(type) {
	case reporting.BudgetResult:
		return budgetBars(th, fmtAmt, d)
	case reporting.PayeesResult:
		items := make([]components.BarItem, len(d.Rows))
		for i, r := range d.Rows {
			items[i] = components.BarItem{Label: r.Payee, Value: r.Total.Minor, DisplayValue: fmtAmt(r.Total)}
		}
		return items
	case reporting.InterestResult:
		items := make([]components.BarItem, len(d.Rows))
		for i, r := range d.Rows {
			items[i] = components.BarItem{
				Label: r.Loan + " " + strconv.Itoa(r.Year), Value: r.Interest.Minor, DisplayValue: fmtAmt(r.Interest),
			}
		}
		return items
	}
	return nil
}

// budgetBars draws two bars per budget: the budgeted amount in the muted color and the spending
// colored by the same 70/90/100 tiers as the Budget screen (Negative once over).
func budgetBars(th theme.Theme, fmtAmt amountFormatter, r reporting.BudgetResult) []components.BarItem {
	items := make([]components.BarItem, 0, 2*len(r.Lines))
	for _, l := range r.Lines {
		items = append(items,
			components.BarItem{Label: l.Budget, Value: l.Budgeted.Minor, DisplayValue: fmtAmt(l.Budgeted), Style: th.Muted},
			components.BarItem{Value: l.Spent.Minor, DisplayValue: fmtAmt(l.Spent), Style: spentStyle(th, l)},
		)
	}
	return items
}

func spentStyle(th theme.Theme, l reporting.BudgetLine) lipgloss.Style {
	if l.Budgeted.Minor <= 0 {
		if l.Spent.Minor > 0 {
			return th.Negative
		}
		return th.Positive
	}
	switch p := l.Spent.Minor * 100 / l.Budgeted.Minor; {
	case p >= 100:
		return th.Negative
	case p >= 90:
		return th.Caution
	case p >= 70:
		return th.Warning
	default:
		return th.Positive
	}
}

// compareBarItems draws period A (Series1) and period B (Series2) side by side per category; a
// report without categories falls back to its total.
func compareBarItems(th theme.Theme, fmtAmt amountFormatter, r reporting.CompareReport) []components.BarItem {
	rows := r.Rows
	if len(rows) == 0 {
		rows = []reporting.CompareRowView{r.Total}
	}
	a := lipgloss.NewStyle().Foreground(th.Tokens.Series1)
	b := lipgloss.NewStyle().Foreground(th.Tokens.Series2)
	items := make([]components.BarItem, 0, 2*len(rows))
	for _, row := range rows {
		label := row.Category
		if label == "" {
			label = "Total"
		}
		items = append(items,
			components.BarItem{Label: label, Value: row.A.Minor, DisplayValue: fmtAmt(row.A), Style: a},
			components.BarItem{Value: row.B.Minor, DisplayValue: fmtAmt(row.B), Style: b},
		)
	}
	return items
}

// renderBars draws at most the rows that fit a screen of height h.
// A non-empty legend is added as one line under the chart.
func (m *Model) renderBars(items []components.BarItem, legend string, width, height int) string {
	n := min(len(items), barRowCap(height))
	if n == 0 {
		return ""
	}
	chart := components.NewBarChart(m.th)
	chart.SetWidth(width)
	chart.SetItems(items[:n])
	out := chart.View()
	if legend != "" {
		out += "\n" + legend
	}
	return out
}

// legendOf renders "■ name" pairs, each swatch in its own style.
func legendOf(entries ...legendEntry) string {
	parts := make([]string, len(entries))
	for i, e := range entries {
		parts[i] = e.style.Render("■") + " " + e.name
	}
	return strings.Join(parts, "  ")
}

type legendEntry struct {
	name  string
	style lipgloss.Style
}

// barLegend is the legend of the paired-bar charts of data, or "" for the others.
func (m *Model) barLegend(data any) string {
	if _, ok := data.(reporting.BudgetResult); ok {
		return legendOf(legendEntry{"budgeted", m.th.Muted}, legendEntry{"spent", m.th.Positive})
	}
	return ""
}

func (m *Model) compareLegend() string {
	return legendOf(
		legendEntry{m.cmp.report.A, lipgloss.NewStyle().Foreground(m.th.Tokens.Series1)},
		legendEntry{m.cmp.report.B, lipgloss.NewStyle().Foreground(m.th.Tokens.Series2)},
	)
}

// barsHeight is the number of lines of a rendered bar chart.
func barsHeight(s string) int {
	if s == "" {
		return 0
	}
	return lipgloss.Height(s) + 1
}

func (m *Model) compareBars() string {
	return m.renderBars(compareBarItems(m.th, m.fmtAmount, m.cmp.report), m.compareLegend(), m.width, m.height)
}
