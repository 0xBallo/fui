package render

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/domain"
)

const chartWidth = 32

// WriteForecast renders the forecast total series.
func WriteForecast(
	w io.Writer, f Format, loc domain.Locale, color bool, out reporting.ForecastResult, chart bool,
) error {
	if f == JSON {
		return WriteJSON(w, out)
	}
	if f == CSV {
		rows := Rows{Header: []string{"DATE", "TOTAL"}, MoneyCols: []int{1}}
		for _, point := range out.Total {
			m, err := moneyFromAmount(point.Amount)
			if err != nil {
				return err
			}
			rows.Body = append(rows.Body, []string{point.Date, FormatMoneyMachine(m)})
		}
		return WriteCSV(w, rows)
	}
	rows := Rows{Header: []string{"DATE", "TOTAL"}, MoneyCols: []int{1}}
	if chart {
		rows.Header = []string{"DATE", "TOTAL", "CHART"}
	}
	bars := barsFromPoints(out.Total)
	for i, point := range out.Total {
		m, err := moneyFromAmount(point.Amount)
		if err != nil {
			return err
		}
		row := []string{point.Date, FormatMoney(m, loc)}
		if chart {
			row = append(row, bars[i])
		}
		rows.Body = append(rows.Body, row)
	}
	return WriteTable(w, rows, color)
}

// WriteSeriesReport renders a date/amount analytics series.
func WriteSeriesReport(
	w io.Writer, f Format, loc domain.Locale, color bool, out reporting.SeriesResult, chart bool,
) error {
	if f == JSON {
		return WriteJSON(w, out)
	}
	if f == CSV {
		rows := Rows{Header: []string{"DATE", "AMOUNT"}, MoneyCols: []int{1}}
		for _, point := range out.Points {
			m, err := moneyFromAmount(point.Amount)
			if err != nil {
				return err
			}
			rows.Body = append(rows.Body, []string{point.Date, FormatMoneyMachine(m)})
		}
		return WriteCSV(w, rows)
	}
	rows := Rows{Header: []string{"DATE", "AMOUNT"}, MoneyCols: []int{1}}
	if chart {
		rows.Header = []string{"DATE", "AMOUNT", "CHART"}
	}
	bars := barsFromPoints(out.Points)
	for i, point := range out.Points {
		m, err := moneyFromAmount(point.Amount)
		if err != nil {
			return err
		}
		row := []string{point.Date, FormatMoney(m, loc)}
		if chart {
			row = append(row, bars[i])
		}
		rows.Body = append(rows.Body, row)
	}
	return WriteTable(w, rows, color)
}

// WriteDebtReport renders the remaining-debt report.
func WriteDebtReport(
	w io.Writer, f Format, loc domain.Locale, color bool, out reporting.DebtResult, chart bool,
) error {
	if f == JSON {
		return WriteJSON(w, out)
	}
	if f == CSV {
		return WriteSeriesReport(w, CSV, loc, color, out.SeriesResult, chart)
	}
	if _, err := fmt.Fprintf(w, "Loan: %s\n", out.Loan); err != nil {
		return fmt.Errorf("write debt title: %w", err)
	}
	return WriteSeriesReport(w, Table, loc, color, out.SeriesResult, chart)
}

// WriteInterestReport renders the interest-paid report.
func WriteInterestReport(
	w io.Writer, f Format, loc domain.Locale, color bool, out reporting.InterestResult, chart bool,
) error {
	if f == JSON {
		return WriteJSON(w, out)
	}
	if f == CSV {
		rows := Rows{Header: []string{"LOAN", "YEAR", "INTEREST", "FEES"}, MoneyCols: []int{2, 3}}
		for _, row := range out.Rows {
			interest, err := moneyFromAmount(row.Interest)
			if err != nil {
				return err
			}
			fees, err := moneyFromAmount(row.Fees)
			if err != nil {
				return err
			}
			rows.Body = append(rows.Body, []string{
				row.Loan, strconv.Itoa(row.Year), FormatMoneyMachine(interest), FormatMoneyMachine(fees),
			})
		}
		return WriteCSV(w, rows)
	}
	rows := Rows{Header: []string{"LOAN", "YEAR", "INTEREST", "FEES"}, MoneyCols: []int{2, 3}}
	values := make([]int64, 0, len(out.Rows))
	for _, row := range out.Rows {
		values = append(values, row.Interest.Minor)
	}
	bars := barStrings(values)
	if chart {
		rows.Header = []string{"LOAN", "YEAR", "INTEREST", "FEES", "CHART"}
	}
	for i, row := range out.Rows {
		interest, err := moneyFromAmount(row.Interest)
		if err != nil {
			return err
		}
		fees, err := moneyFromAmount(row.Fees)
		if err != nil {
			return err
		}
		body := []string{row.Loan, strconv.Itoa(row.Year), FormatMoney(interest, loc), FormatMoney(fees, loc)}
		if chart {
			body = append(body, bars[i])
		}
		rows.Body = append(rows.Body, body)
	}
	return WriteTable(w, rows, color)
}

// WritePayeesReport renders the top-payees report.
func WritePayeesReport(
	w io.Writer, f Format, loc domain.Locale, color bool, out reporting.PayeesResult, chart bool,
) error {
	if f == JSON {
		return WriteJSON(w, out)
	}
	if f == CSV {
		rows := Rows{Header: []string{"PAYEE", "TOTAL", "COUNT"}, MoneyCols: []int{1}}
		for _, row := range out.Rows {
			total, err := moneyFromAmount(row.Total)
			if err != nil {
				return err
			}
			rows.Body = append(rows.Body, []string{row.Payee, FormatMoneyMachine(total), strconv.Itoa(row.Count)})
		}
		totalMoney, err := moneyFromAmount(out.Total)
		if err != nil {
			return err
		}
		rows.Body = append(rows.Body, []string{"TOTAL", FormatMoneyMachine(totalMoney), strconv.Itoa(len(out.Rows))})
		return WriteCSV(w, rows)
	}
	rows := Rows{Header: []string{"PAYEE", "TOTAL", "COUNT"}, MoneyCols: []int{1}}
	values := make([]int64, 0, len(out.Rows))
	for _, row := range out.Rows {
		values = append(values, row.Total.Minor)
	}
	bars := barStrings(values)
	if chart {
		rows.Header = []string{"PAYEE", "TOTAL", "COUNT", "CHART"}
	}
	for i, row := range out.Rows {
		total, err := moneyFromAmount(row.Total)
		if err != nil {
			return err
		}
		body := []string{row.Payee, FormatMoney(total, loc), strconv.Itoa(row.Count)}
		if chart {
			body = append(body, bars[i])
		}
		rows.Body = append(rows.Body, body)
	}
	totalMoney, err := moneyFromAmount(out.Total)
	if err != nil {
		return err
	}
	totalRow := []string{"TOTAL", FormatMoney(totalMoney, loc), strconv.Itoa(len(out.Rows))}
	if chart {
		totalRow = append(totalRow, "")
	}
	rows.Body = append(rows.Body, totalRow)
	return WriteTable(w, rows, color)
}

// WriteBudgetReport renders the budget-vs-actual report.
func WriteBudgetReport(
	w io.Writer, f Format, loc domain.Locale, color bool, out reporting.BudgetResult, chart bool,
) error {
	if f == JSON {
		return WriteJSON(w, out)
	}
	if f == CSV {
		return writeBudgetCSV(w, out)
	}
	return writeBudgetTable(w, loc, color, out, chart)
}

func writeBudgetCSV(w io.Writer, out reporting.BudgetResult) error {
	rows := Rows{Header: []string{"BUDGET", "BUDGETED", "SPENT", "REMAINING", "STATE"}, MoneyCols: []int{1, 2, 3}}
	for _, line := range out.Lines {
		budgeted, err := moneyFromAmount(line.Budgeted)
		if err != nil {
			return err
		}
		spent, err := moneyFromAmount(line.Spent)
		if err != nil {
			return err
		}
		remaining, err := moneyFromAmount(line.Remaining)
		if err != nil {
			return err
		}
		rows.Body = append(rows.Body, []string{
			line.Budget, FormatMoneyMachine(budgeted), FormatMoneyMachine(spent), FormatMoneyMachine(remaining), line.State,
		})
	}
	return WriteCSV(w, rows)
}

func writeBudgetTable(w io.Writer, loc domain.Locale, color bool, out reporting.BudgetResult, chart bool) error {
	rows := Rows{Header: []string{"BUDGET", "BUDGETED", "SPENT", "REMAINING", "STATE"}, MoneyCols: []int{1, 2, 3}}
	values := make([]int64, 0, len(out.Lines))
	for _, line := range out.Lines {
		values = append(values, line.Spent.Minor)
	}
	bars := barStrings(values)
	if chart {
		rows.Header = []string{"BUDGET", "BUDGETED", "SPENT", "REMAINING", "STATE", "CHART"}
	}
	for i, line := range out.Lines {
		budgeted, err := moneyFromAmount(line.Budgeted)
		if err != nil {
			return err
		}
		spent, err := moneyFromAmount(line.Spent)
		if err != nil {
			return err
		}
		remaining, err := moneyFromAmount(line.Remaining)
		if err != nil {
			return err
		}
		body := []string{
			line.Budget,
			FormatMoney(budgeted, loc),
			FormatMoney(spent, loc),
			FormatMoney(remaining, loc),
			line.State,
		}
		if chart {
			body = append(body, bars[i])
		}
		rows.Body = append(rows.Body, body)
	}
	return WriteTable(w, rows, color)
}

// WriteSavedReport renders one saved report definition.
func WriteSavedReport(w io.Writer, f Format, loc domain.Locale, color bool, saved app.SavedReport) error {
	if f == JSON {
		return WriteJSON(w, toSavedReportDTO(saved))
	}
	return WriteSavedReportList(w, f, loc, color, []app.SavedReport{saved})
}

// WriteSavedReportList renders saved report definitions.
func WriteSavedReportList(w io.Writer, f Format, _ domain.Locale, color bool, rows []app.SavedReport) error {
	if f == JSON {
		out := make([]savedReportDTO, len(rows))
		for i, row := range rows {
			out[i] = toSavedReportDTO(row)
		}
		return WriteJSON(w, out)
	}
	table := Rows{Header: []string{"NAME", "KIND", "PARAMS"}}
	for _, row := range rows {
		table.Body = append(table.Body, []string{row.Name, row.Kind, row.ParamsJSON})
	}
	return WriteTable(w, table, color)
}

type savedReportDTO struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	ParamsJSON string `json:"params_json"`
}

func toSavedReportDTO(row app.SavedReport) savedReportDTO {
	return savedReportDTO{
		ID: int64(row.ID), Name: row.Name, Kind: row.Kind, ParamsJSON: row.ParamsJSON,
	}
}

// WriteSavedReportResult renders the result of running a saved report.
func WriteSavedReportResult(
	w io.Writer, f Format, loc domain.Locale, color bool, out reporting.SavedResult, chart bool,
) error {
	if f == JSON {
		return WriteJSON(w, out)
	}
	switch data := out.Data.(type) {
	case reporting.SeriesResult:
		return WriteSeriesReport(w, Table, loc, color, data, chart)
	case reporting.DebtResult:
		return WriteDebtReport(w, Table, loc, color, data, chart)
	case reporting.InterestResult:
		return WriteInterestReport(w, Table, loc, color, data, chart)
	case reporting.PayeesResult:
		return WritePayeesReport(w, Table, loc, color, data, chart)
	case reporting.BudgetResult:
		return WriteBudgetReport(w, Table, loc, color, data, chart)
	case reporting.ForecastResult:
		return WriteForecast(w, Table, loc, color, data, chart)
	default:
		return fmt.Errorf("render saved report %q (%s): unsupported type %T", out.Name, out.Kind, out.Data)
	}
}

func barsFromPoints(points []reporting.Point) []string {
	values := make([]int64, len(points))
	for i, point := range points {
		values[i] = point.Amount.Minor
	}
	return barStrings(values)
}

func barStrings(values []int64) []string {
	if len(values) == 0 {
		return nil
	}
	var maxAbs int64
	for _, value := range values {
		abs := int64(math.Abs(float64(value)))
		if abs > maxAbs {
			maxAbs = abs
		}
	}
	out := make([]string, len(values))
	if maxAbs == 0 {
		return out
	}
	for i, value := range values {
		abs := int64(math.Abs(float64(value)))
		fill := int(float64(abs) / float64(maxAbs) * chartWidth)
		if fill == 0 && abs > 0 {
			fill = 1
		}
		out[i] = strings.Repeat("█", fill)
	}
	return out
}

func moneyFromAmount(a reporting.Amount) (domain.Money, error) {
	currency, err := domain.ParseCurrency(a.Currency)
	if err != nil {
		return domain.Money{}, fmt.Errorf("parse currency %q: %w", a.Currency, err)
	}
	money, err := domain.NewMoney(a.Minor, currency)
	if err != nil {
		return domain.Money{}, err
	}
	return money, nil
}
