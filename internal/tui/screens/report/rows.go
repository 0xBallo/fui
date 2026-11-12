package report

import (
	"strconv"

	"github.com/charmbracelet/lipgloss"

	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/components"
)

// chartCap is the largest row count the bar chart renders; longer series (e.g. a 90-day
// forecast) stay in the scrollable table only.
const chartCap = 40

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
	m.chart.SetItems(nil)
	switch data := m.result.data.(type) {
	case reporting.SeriesResult:
		m.viewSeries(data)
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
	items := make([]components.BarItem, 0, len(s.Points))
	for i, p := range s.Points {
		rows[i] = components.Row{ID: p.Date, Cells: []string{p.Date, m.fmtAmount(p.Amount)}}
		items = append(items, components.BarItem{Label: p.Date, Value: p.Amount.Minor, DisplayValue: m.fmtAmount(p.Amount)})
	}
	m.table.SetRows(rows)
	m.setChart(items)
}

func (m *Model) viewInterest(r reporting.InterestResult) {
	m.table.SetColumns([]components.Column{
		{Title: "Loan", Width: 18, Align: lipgloss.Left},
		{Title: "Year", Width: 6, Align: lipgloss.Right},
		{Title: "Interest", Width: 14, Align: lipgloss.Right},
		{Title: "Fees", Width: 14, Align: lipgloss.Right},
	})
	rows := make([]components.Row, len(r.Rows))
	items := make([]components.BarItem, 0, len(r.Rows))
	for i, row := range r.Rows {
		year := strconv.Itoa(row.Year)
		rows[i] = components.Row{ID: row.Loan + year, Cells: []string{
			row.Loan, year, m.fmtAmount(row.Interest), m.fmtAmount(row.Fees),
		}}
		items = append(items, components.BarItem{Label: row.Loan + " " + year, Value: row.Interest.Minor, DisplayValue: m.fmtAmount(row.Interest)})
	}
	m.table.SetRows(rows)
	m.setChart(items)
}

func (m *Model) viewPayees(r reporting.PayeesResult) {
	m.table.SetColumns([]components.Column{
		{Title: "Payee", Width: 20, Align: lipgloss.Left},
		{Title: "Total", Width: 14, Align: lipgloss.Right},
		{Title: "Count", Width: 7, Align: lipgloss.Right},
	})
	rows := make([]components.Row, 0, len(r.Rows)+1)
	items := make([]components.BarItem, 0, len(r.Rows))
	for _, row := range r.Rows {
		rows = append(rows, components.Row{ID: row.Payee, Cells: []string{row.Payee, m.fmtAmount(row.Total), strconv.Itoa(row.Count)}})
		items = append(items, components.BarItem{Label: row.Payee, Value: row.Total.Minor, DisplayValue: m.fmtAmount(row.Total)})
	}
	rows = append(rows, components.Row{ID: "TOTAL", Cells: []string{"TOTAL", m.fmtAmount(r.Total), strconv.Itoa(len(r.Rows))}})
	m.table.SetRows(rows)
	m.setChart(items)
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
	items := make([]components.BarItem, 0, len(r.Lines))
	for i, l := range r.Lines {
		rows[i] = components.Row{ID: l.Budget, Cells: []string{
			l.Budget, m.fmtAmount(l.Budgeted), m.fmtAmount(l.Spent), m.fmtAmount(l.Remaining), l.State,
		}}
		items = append(items, components.BarItem{Label: l.Budget, Value: l.Spent.Minor, DisplayValue: m.fmtAmount(l.Spent)})
	}
	m.table.SetRows(rows)
	m.setChart(items)
	m.note = "Uncovered: " + m.fmtAmount(r.Uncovered)
}

func (m *Model) viewForecast(f reporting.ForecastResult) {
	m.table.SetColumns([]components.Column{
		{Title: "Date", Width: 14, Align: lipgloss.Left},
		{Title: "Total", Width: 16, Align: lipgloss.Right},
	})
	rows := make([]components.Row, len(f.Total))
	items := make([]components.BarItem, 0, len(f.Total))
	for i, p := range f.Total {
		rows[i] = components.Row{ID: p.Date, Cells: []string{p.Date, m.fmtAmount(p.Amount)}}
		items = append(items, components.BarItem{Label: p.Date, Value: p.Amount.Minor, DisplayValue: m.fmtAmount(p.Amount)})
	}
	m.table.SetRows(rows)
	m.setChart(items)
	var notes []string
	if f.FirstBelowZero != "" {
		notes = append(notes, "first below zero: "+f.FirstBelowZero)
	}
	if f.FirstBelowMin != "" {
		notes = append(notes, "first below minimum: "+f.FirstBelowMin)
	}
	m.note = joinNotes(notes)
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

// setChart feeds items into the bar chart, skipping charts too long to read.
func (m *Model) setChart(items []components.BarItem) {
	if len(items) > chartCap {
		m.chart.SetItems(nil)
		return
	}
	m.chart.SetItems(items)
}
