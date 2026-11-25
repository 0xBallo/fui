package render

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/domain"
)

// WriteReportResult renders the result of any analytics report as CSV or JSON, dispatching on the
// dynamic type of data; a reporting.SavedResult renders its Data. It is the machine-readable
// entry point for callers that hold a result without knowing its report kind.
func WriteReportResult(w io.Writer, f Format, loc domain.Locale, data any) error {
	if f != CSV && f != JSON {
		return invalidRender("format", "report results render only as csv or json")
	}
	if saved, ok := data.(reporting.SavedResult); ok {
		data = saved.Data
	}
	return writeResultData(w, f, loc, false, false, data)
}

// writeResultData dispatches on the type of an analytics report result.
func writeResultData(w io.Writer, f Format, loc domain.Locale, color, chart bool, data any) error {
	switch d := data.(type) {
	case reporting.SeriesResult:
		return WriteSeriesReport(w, f, loc, color, d, chart)
	case reporting.DebtResult:
		return WriteDebtReport(w, f, loc, color, d, chart)
	case reporting.InterestResult:
		return WriteInterestReport(w, f, loc, color, d, chart)
	case reporting.PayeesResult:
		return WritePayeesReport(w, f, loc, color, d, chart)
	case reporting.BudgetResult:
		return WriteBudgetReport(w, f, loc, color, d, chart)
	case reporting.ForecastResult:
		return WriteForecast(w, f, loc, color, d, chart)
	case reporting.NetWorthResult:
		return WriteNetWorthReport(w, f, loc, color, d, chart)
	case reporting.CompareReport:
		return WriteCompareReport(w, f, loc, color, d)
	case reporting.GroupedResult:
		return WriteGroupedReport(w, f, loc, color, d, chart)
	default:
		return invalidRender("report", fmt.Sprintf("unsupported report result type %T", data))
	}
}

func invalidRender(field, msg string) error {
	return &domain.Error{Code: domain.CodeInvalidValue, Message: msg, Details: map[string]any{"field": field}}
}

// amountCell formats a report amount for the table (localized) or CSV (machine) output.
func amountCell(a reporting.Amount, f Format, loc domain.Locale) (string, error) {
	m, err := moneyFromAmount(a)
	if err != nil {
		return "", err
	}
	if f == CSV {
		return FormatMoneyMachine(m), nil
	}
	return FormatMoney(m, loc), nil
}

func optAmountCell(a *reporting.Amount, f Format, loc domain.Locale) (string, error) {
	if a == nil {
		return "", nil
	}
	return amountCell(*a, f, loc)
}

func writeRows(w io.Writer, f Format, color bool, rows Rows) error {
	if f == CSV {
		return WriteCSV(w, rows)
	}
	return WriteTable(w, rows, color)
}

// WriteGroupedReport renders a spending or cash-flow report grouped by one dimension: one row
// per group plus a TOTAL row. Cash-flow rows add inflow and outflow columns; chart adds a bar
// column on the row totals in table output.
func WriteGroupedReport(
	w io.Writer, f Format, loc domain.Locale, color bool, out reporting.GroupedResult, chart bool,
) error {
	if f == JSON {
		return WriteJSON(w, out)
	}
	flows := len(out.Rows) > 0 && out.Rows[0].Inflow != nil
	showChart := chart && f == Table
	rows := groupedHeader(out.GroupBy, flows, showChart)
	totals := make([]int64, len(out.Rows))
	for i, r := range out.Rows {
		totals[i] = r.Total.Minor
	}
	bars := barStrings(totals)
	for i, r := range out.Rows {
		cells, err := groupedLine(r.Key, r, flows, f, loc)
		if err != nil {
			return err
		}
		if showChart {
			cells = append(cells, bars[i])
		}
		rows.Body = append(rows.Body, cells)
	}
	cells, err := groupedLine("TOTAL", reporting.GroupedRow{Total: out.Total}, flows, f, loc)
	if err != nil {
		return err
	}
	if showChart {
		cells = append(cells, "")
	}
	rows.Body = append(rows.Body, cells)
	return writeRows(w, f, color, rows)
}

func groupedHeader(groupBy string, flows, chart bool) Rows {
	name := strings.ToUpper(groupBy)
	if name == "" {
		name = "GROUP"
	}
	rows := Rows{Header: []string{name, "TOTAL"}, MoneyCols: []int{1}}
	if flows {
		rows = Rows{Header: []string{name, "INFLOW", "OUTFLOW", "NET"}, MoneyCols: []int{1, 2, 3}}
	}
	if chart {
		rows.Header = append(rows.Header, "CHART")
	}
	return rows
}

func groupedLine(key string, r reporting.GroupedRow, flows bool, f Format, loc domain.Locale) ([]string, error) {
	cells := []string{key}
	if flows {
		for _, a := range []*reporting.Amount{r.Inflow, r.Outflow} {
			c, err := optAmountCell(a, f, loc)
			if err != nil {
				return nil, err
			}
			cells = append(cells, c)
		}
	}
	c, err := amountCell(r.Total, f, loc)
	if err != nil {
		return nil, err
	}
	return append(cells, c), nil
}

// WriteNetWorthReport renders the net-worth series with its components. A component of a
// disabled module is empty in CSV and table output and omitted in JSON.
func WriteNetWorthReport(
	w io.Writer, f Format, loc domain.Locale, color bool, out reporting.NetWorthResult, chart bool,
) error {
	if f == JSON {
		return WriteJSON(w, out)
	}
	header := []string{"DATE", "LIQUIDITY", "POSITIONS", "ASSETS", "DEBTS", "TOTAL"}
	showChart := chart && f == Table
	if showChart {
		header = append(header, "CHART")
	}
	rows := Rows{Header: header, MoneyCols: []int{1, 2, 3, 4, 5}}
	totals := make([]int64, len(out.Points))
	for i, p := range out.Points {
		totals[i] = p.Total.Minor
	}
	bars := barStrings(totals)
	for i, p := range out.Points {
		cells := []string{p.Date}
		liq, err := amountCell(p.Liquidity, f, loc)
		if err != nil {
			return err
		}
		cells = append(cells, liq)
		for _, a := range []*reporting.Amount{p.Positions, p.Assets, p.Debts} {
			c, err := optAmountCell(a, f, loc)
			if err != nil {
				return err
			}
			cells = append(cells, c)
		}
		total, err := amountCell(p.Total, f, loc)
		if err != nil {
			return err
		}
		cells = append(cells, total)
		if showChart {
			cells = append(cells, bars[i])
		}
		rows.Body = append(rows.Body, cells)
	}
	return writeRows(w, f, color, rows)
}

// WriteCompareReport renders the comparison of two periods: one row per category and a TOTAL row.
func WriteCompareReport(
	w io.Writer, f Format, loc domain.Locale, color bool, out reporting.CompareReport,
) error {
	if f == JSON {
		return WriteJSON(w, out)
	}
	rows := Rows{Header: []string{"CATEGORY", "A", "B", "DELTA", "DELTA %"}, MoneyCols: []int{1, 2, 3}}
	all := append(append([]reporting.CompareRowView{}, out.Rows...), out.Total)
	for i, r := range all {
		name := r.Category
		if i == len(all)-1 {
			name = "TOTAL"
		}
		cells := []string{name}
		for _, amt := range []reporting.Amount{r.A, r.B, r.Delta} {
			s, err := amountCell(amt, f, loc)
			if err != nil {
				return err
			}
			cells = append(cells, s)
		}
		cells = append(cells, permilleText(r.DeltaPermille))
		rows.Body = append(rows.Body, cells)
	}
	return writeRows(w, f, color, rows)
}

// permilleText renders a permille change as a percentage with one decimal; "-" when undefined.
func permilleText(p *int64) string {
	if p == nil {
		return "-"
	}
	n := *p
	sign := ""
	if n < 0 {
		sign, n = "-", -n
	}
	return sign + strconv.FormatInt(n/10, 10) + "." + strconv.FormatInt(n%10, 10) + "%"
}
