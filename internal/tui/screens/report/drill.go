package report

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/screens"
)

// periodRange splits the "from..to" period text of a report result.
func periodRange(text string) (from, to string, ok bool) {
	from, to, ok = strings.Cut(text, "..")
	return from, to, ok && from != "" && to != ""
}

// dayBefore returns the day before the ISO date d.
func dayBefore(d string) (string, error) {
	date, err := domain.ParseDate(d)
	if err != nil {
		return "", err
	}
	prev, err := date.AddDays(-1)
	if err != nil {
		return "", err
	}
	return prev.String(), nil
}

// dayAfter returns the day after the ISO date d.
func dayAfter(d string) (string, error) {
	date, err := domain.ParseDate(d)
	if err != nil {
		return "", err
	}
	next, err := date.AddDays(1)
	if err != nil {
		return "", err
	}
	return next.String(), nil
}

// drillable reports whether the loaded report has rows that open the transactions behind them.
func (m *Model) drillable() bool {
	switch d := m.result.data.(type) {
	case reporting.GroupedResult:
		return len(d.Rows) > 0
	case reporting.PayeesResult:
		return len(d.Rows) > 0
	case reporting.SeriesResult:
		return len(d.Points) > 0
	case reporting.NetWorthResult:
		return len(d.Points) > 0
	}
	return false
}

// drill opens the Transactions tab narrowed to the selected row of the report.
func (m *Model) drill() tea.Cmd {
	if !m.drillable() {
		return nil
	}
	req, ok := m.drillFilter(m.table.Cursor())
	if !ok {
		return nil
	}
	req = mergeFilter(m.result.filter, req)
	return func() tea.Msg { return screens.SwitchScreenMsg{Target: screens.TabTransactions, Filter: &req} }
}

// drillFilter builds the transaction filter of row i of the loaded report: the report period (or
// the bucket of the row) plus the category, payee, account or tag the row stands for. The TOTAL
// row has no dimension, so it drills into the whole period.
func (m *Model) drillFilter(i int) (app.ListTxRequest, bool) {
	switch d := m.result.data.(type) {
	case reporting.GroupedResult:
		return m.groupedFilter(d, i)
	case reporting.PayeesResult:
		from, to, ok := periodRange(d.Period)
		req := app.ListTxRequest{From: from, To: to}
		if i < len(d.Rows) {
			req.Payees = []string{d.Rows[i].Payee}
		}
		return req, ok
	case reporting.SeriesResult:
		req, ok := m.pointFilter(d.Period, seriesDates(d), i)
		if ok && m.result.saved == "" && m.accountText != "" && m.result.kind == reporting.KindBalance {
			req.Accounts = []string{m.accountText}
		}
		return req, ok
	case reporting.NetWorthResult:
		return m.pointFilter(d.Period, seriesDates(d.Series()), i)
	}
	return app.ListTxRequest{}, false
}

func seriesDates(s reporting.SeriesResult) []string {
	dates := make([]string, len(s.Points))
	for i, p := range s.Points {
		dates[i] = p.Date
	}
	return dates
}

// pointFilter maps the point i of a series, dated at the end of its bucket, to that bucket.
func (m *Model) pointFilter(period string, dates []string, i int) (app.ListTxRequest, bool) {
	from, _, ok := periodRange(period)
	if !ok || i < 0 || i >= len(dates) {
		return app.ListTxRequest{}, false
	}
	if i > 0 {
		var err error
		if from, err = dayAfter(dates[i-1]); err != nil {
			return app.ListTxRequest{}, false
		}
	}
	return app.ListTxRequest{From: from, To: dates[i]}, true
}

func (m *Model) groupedFilter(g reporting.GroupedResult, i int) (app.ListTxRequest, bool) {
	from, to, ok := periodRange(g.Period)
	if !ok {
		return app.ListTxRequest{}, false
	}
	req := app.ListTxRequest{From: from, To: to}
	if m.result.kind != reporting.KindCashFlow && !isCashFlow(g) {
		req.Sign = domain.TxSignExpense
	}
	if i < 0 || i >= len(g.Rows) {
		return req, i == len(g.Rows)
	}
	key := g.Rows[i].Key
	switch by := domain.ReportGroup(g.GroupBy); by {
	case domain.GroupCategory, domain.GroupPayee, domain.GroupAccount, domain.GroupTag:
		dimensionFilter(&req, by, key)
	case domain.GroupWeek, domain.GroupMonth, domain.GroupYear:
		req.From = key
		if i+1 < len(g.Rows) {
			end, err := dayBefore(g.Rows[i+1].Key)
			if err != nil {
				return app.ListTxRequest{}, false
			}
			req.To = end
		}
	}
	return req, true
}

// dimensionFilter narrows req to the group key of a non-period dimension; the labels of the
// rows that collect "none" (no payee, untagged) carry no filter.
func dimensionFilter(req *app.ListTxRequest, by domain.ReportGroup, key string) {
	switch by {
	case domain.GroupCategory:
		if key == reporting.UncategorizedCategoryName {
			req.Uncategorized = true
		} else {
			req.Categories = []string{key}
		}
	case domain.GroupPayee:
		if key != reporting.NoPayeeLabel {
			req.Payees = []string{key}
		}
	case domain.GroupAccount:
		req.Accounts = []string{key}
	case domain.GroupTag:
		if key != reporting.UntaggedLabel {
			req.Tags = []string{key}
		}
	case domain.GroupWeek, domain.GroupMonth, domain.GroupYear:
	}
}

// compareFilter builds the filter of row i of the comparison: period B and, for a category row,
// that category. The last row is the TOTAL.
func (m *Model) compareFilter(i int) (app.ListTxRequest, bool) {
	rep := m.cmp.report
	from, to, ok := periodRange(rep.B)
	if !ok || i < 0 || i > len(rep.Rows) {
		return app.ListTxRequest{}, false
	}
	req := app.ListTxRequest{From: from, To: to, Sign: domain.TxSignExpense}
	if i < len(rep.Rows) {
		if c := rep.Rows[i].Category; c == reporting.UncategorizedCategoryName {
			req.Uncategorized = true
		} else {
			req.Categories = []string{c}
		}
	}
	return req, true
}

// drillCompare opens the Transactions tab on the selected comparison row.
func (m *Model) drillCompare() tea.Cmd {
	if !m.cmp.loaded {
		return nil
	}
	req, ok := m.compareFilter(m.cmp.table.Cursor())
	if !ok {
		return nil
	}
	return func() tea.Msg { return screens.SwitchScreenMsg{Target: screens.TabTransactions, Filter: &req} }
}
