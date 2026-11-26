package report

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/components"
)

// moneyOf rebuilds a domain.Money from a reporting.Amount, as the CLI render package does.
func moneyOf(a reporting.Amount) (domain.Money, error) {
	cur, err := domain.ParseCurrency(a.Currency)
	if err != nil {
		return domain.Money{}, err
	}
	return domain.NewMoney(a.Minor, cur)
}

// fmtAmount formats a reporting.Amount with loc, or "?" if it carries an invalid currency (an
// empty, never-set Amount).
func (m *Model) fmtAmount(a reporting.Amount) string {
	money, err := moneyOf(a)
	if err != nil {
		return "?"
	}
	return domain.FormatAmount(money, m.loc)
}

// refreshResultView rebuilds the table, chart and note from the currently loaded result.
func (m *Model) refreshResultView() {
	m.note = ""
	m.sizeReportTable()
	switch data := m.result.data.(type) {
	case reporting.SeriesResult:
		m.viewSeries(data)
	case reporting.NetWorthResult:
		m.viewSeries(data.Series())
	case reporting.DebtResult:
		m.note = "Loan: " + data.Loan
		m.viewSeries(data.SeriesResult)
	case reporting.InterestResult:
		m.viewInterest(data)
	case reporting.PayeesResult:
		m.viewPayees(data)
	case reporting.BudgetResult:
		m.viewBudget(data)
	case reporting.ForecastResult:
		m.viewForecast(data)
	case reporting.GroupedResult:
		m.viewGrouped(data)
	default:
		m.table.SetColumns(nil)
		m.table.SetRows(nil)
		m.note = "This report kind has no TUI view yet; use `fui report` for it."
	}
}

func (m *Model) viewSeries(s reporting.SeriesResult) {
	m.table.SetColumns([]components.Column{
		{Title: "Date", Width: 14, Align: lipgloss.Left},
		{Title: "Amount", Width: 16, Align: lipgloss.Right},
	})
	rows := make([]components.Row, len(s.Points))
	for i, p := range s.Points {
		rows[i] = components.Row{ID: p.Date, Cells: []string{p.Date, m.fmtAmount(p.Amount)}}
	}
	m.table.SetRows(rows)
}

func (m *Model) viewInterest(r reporting.InterestResult) {
	m.table.SetColumns([]components.Column{
		{Title: "Loan", Width: 18, Align: lipgloss.Left},
		{Title: "Year", Width: 6, Align: lipgloss.Right},
		{Title: "Interest", Width: 14, Align: lipgloss.Right},
		{Title: "Fees", Width: 14, Align: lipgloss.Right},
	})
	rows := make([]components.Row, len(r.Rows))
	for i, row := range r.Rows {
		year := strconv.Itoa(row.Year)
		rows[i] = components.Row{ID: row.Loan + year, Cells: []string{
			row.Loan, year, m.fmtAmount(row.Interest), m.fmtAmount(row.Fees),
		}}
	}
	m.table.SetRows(rows)
}

func (m *Model) viewPayees(r reporting.PayeesResult) {
	m.table.SetColumns([]components.Column{
		{Title: "Payee", Width: 20, Align: lipgloss.Left},
		{Title: "Total", Width: 14, Align: lipgloss.Right},
		{Title: "Count", Width: 7, Align: lipgloss.Right},
	})
	rows := make([]components.Row, 0, len(r.Rows)+1)
	for _, row := range r.Rows {
		rows = append(rows, components.Row{ID: row.Payee, Cells: []string{row.Payee, m.fmtAmount(row.Total), strconv.Itoa(row.Count)}})
	}
	rows = append(rows, components.Row{ID: "TOTAL", Cells: []string{"TOTAL", m.fmtAmount(r.Total), strconv.Itoa(len(r.Rows))}})
	m.table.SetRows(rows)
}

func (m *Model) viewBudget(r reporting.BudgetResult) {
	m.table.SetColumns([]components.Column{
		{Title: "Budget", Width: 18, Align: lipgloss.Left},
		{Title: "Budgeted", Width: 12, Align: lipgloss.Right},
		{Title: "Spent", Width: 12, Align: lipgloss.Right},
		{Title: "Remaining", Width: 12, Align: lipgloss.Right},
		{Title: "State", Width: 8, Align: lipgloss.Left},
	})
	rows := make([]components.Row, len(r.Lines))
	for i, l := range r.Lines {
		rows[i] = components.Row{ID: l.Budget, Cells: []string{
			l.Budget, m.fmtAmount(l.Budgeted), m.fmtAmount(l.Spent), m.fmtAmount(l.Remaining), l.State,
		}}
	}
	m.table.SetRows(rows)
	m.note = "Uncovered: " + m.fmtAmount(r.Uncovered)
}

func (m *Model) viewForecast(f reporting.ForecastResult) {
	m.table.SetColumns([]components.Column{
		{Title: "Date", Width: 14, Align: lipgloss.Left},
		{Title: "Total", Width: 16, Align: lipgloss.Right},
	})
	rows := make([]components.Row, len(f.Total))
	for i, p := range f.Total {
		rows[i] = components.Row{ID: p.Date, Cells: []string{p.Date, m.fmtAmount(p.Amount)}}
	}
	m.table.SetRows(rows)
	var notes []string
	if f.FirstBelowZero != "" {
		notes = append(notes, "first below zero: "+f.FirstBelowZero)
	}
	if f.FirstBelowMin != "" {
		notes = append(notes, "first below minimum: "+f.FirstBelowMin)
	}
	m.note = joinNotes(notes)
}

// viewGrouped shows a spending or cash-flow result: spending rows carry only a total, cash-flow
// rows add the inflow and outflow next to the net.
func (m *Model) viewGrouped(g reporting.GroupedResult) {
	cashFlow := m.result.kind == reporting.KindCashFlow || isCashFlow(g)
	cols := []components.Column{
		{Title: groupTitle(g.GroupBy), Width: 22, Align: lipgloss.Left},
		{Title: "Total", Width: 14, Align: lipgloss.Right},
	}
	if cashFlow {
		cols = []components.Column{
			{Title: groupTitle(g.GroupBy), Width: 22, Align: lipgloss.Left},
			{Title: "Inflow", Width: 14, Align: lipgloss.Right},
			{Title: "Outflow", Width: 14, Align: lipgloss.Right},
			{Title: "Net", Width: 14, Align: lipgloss.Right},
		}
	}
	m.table.SetColumns(cols)
	rows := make([]components.Row, 0, len(g.Rows)+1)
	for _, r := range g.Rows {
		cells := []string{r.Key, m.fmtAmount(r.Total)}
		if cashFlow {
			cells = []string{r.Key, m.optAmount(r.Inflow), m.optAmount(r.Outflow), m.fmtAmount(r.Total)}
		}
		rows = append(rows, components.Row{ID: r.Key, Cells: cells})
	}
	total := []string{"TOTAL", m.fmtAmount(g.Total)}
	if cashFlow {
		total = []string{"TOTAL", "", "", m.fmtAmount(g.Total)}
	}
	rows = append(rows, components.Row{ID: "TOTAL", Cells: total})
	m.table.SetRows(rows)
}

func (m *Model) optAmount(a *reporting.Amount) string {
	if a == nil {
		return ""
	}
	return m.fmtAmount(*a)
}

// groupTitle is the first column header of a grouped report.
func groupTitle(by string) string {
	if by == "" {
		return "Group"
	}
	return strings.ToUpper(by[:1]) + by[1:]
}

func joinNotes(notes []string) string {
	out := ""
	for i, n := range notes {
		if i > 0 {
			out += "   "
		}
		out += n
	}
	return out
}

// sizeReportTable shrinks the table by the line chart rows when the loaded kind draws one.
func (m *Model) sizeReportTable() {
	h := max(3, m.height-14)
	if ch := chartHeight(m.height); ch > 0 && len(lineSeries(m.result.kind, m.result.data)) > 0 {
		h = max(3, m.height-8-ch)
	}
	if n := min(len(barItems(m.th, m.fmtAmount, m.result.data)), barRowCap(m.height)); n > 0 {
		if m.barLegend(m.result.data) != "" {
			n++
		}
		h = max(3, m.height-9-n)
	}
	m.table.SetHeight(h)
}
