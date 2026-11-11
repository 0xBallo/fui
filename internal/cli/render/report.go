package render

import (
	"io"
	"strconv"

	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/domain"
)

// SpendingRowDTO is the --json shape of one category line of a spending report.
type SpendingRowDTO struct {
	CategoryID    int64  `json:"category_id,omitempty"`
	CategoryName  string `json:"category_name"`
	Uncategorized bool   `json:"uncategorized,omitempty"`
	Total         int64  `json:"total"`
}

// SpendingDTO is the --json shape of a spending-by-category report.
type SpendingDTO struct {
	From     string           `json:"from"`
	To       string           `json:"to"`
	Currency string           `json:"currency"`
	Rows     []SpendingRowDTO `json:"rows"`
	Total    int64            `json:"total"`
}

func spendingDTO(r reporting.SpendingResult) SpendingDTO {
	rows := make([]SpendingRowDTO, len(r.Rows))
	for i, row := range r.Rows {
		rows[i] = SpendingRowDTO{
			CategoryID: int64(row.CategoryID), CategoryName: row.CategoryName,
			Uncategorized: row.Uncategorized, Total: row.Total.Minor(),
		}
	}
	return SpendingDTO{
		From: r.Period.From().String(), To: r.Period.To().String(), Currency: r.Currency.Code(),
		Rows: rows, Total: r.Total.Minor(),
	}
}

func spendingRows(r reporting.SpendingResult, loc domain.Locale) Rows {
	t := Rows{Header: []string{"CATEGORY", "TOTAL"}, MoneyCols: []int{1}}
	for _, row := range r.Rows {
		t.Body = append(t.Body, []string{row.CategoryName, FormatMoney(row.Total, loc)})
	}
	t.Body = append(t.Body, []string{"TOTAL", FormatMoney(r.Total, loc)})
	return t
}

// WriteSpending renders the spending-by-category report.
func WriteSpending(w io.Writer, f Format, loc domain.Locale, color bool, r reporting.SpendingResult) error {
	if f == JSON {
		return WriteJSON(w, spendingDTO(r))
	}
	if f == CSV {
		return WriteCSV(w, spendingRows(r, loc))
	}
	return WriteTable(w, spendingRows(r, loc), color)
}

// CashFlowMonthDTO is the --json shape of one month of a cash-flow report.
type CashFlowMonthDTO struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Inflow  int64  `json:"inflow"`
	Outflow int64  `json:"outflow"`
	Net     int64  `json:"net"`
}

// CashFlowDTO is the --json shape of a cash-flow report.
type CashFlowDTO struct {
	From         string             `json:"from"`
	To           string             `json:"to"`
	Currency     string             `json:"currency"`
	Months       []CashFlowMonthDTO `json:"months"`
	TotalInflow  int64              `json:"total_inflow"`
	TotalOutflow int64              `json:"total_outflow"`
	TotalNet     int64              `json:"total_net"`
}

func cashFlowDTO(r reporting.CashFlowResult) CashFlowDTO {
	months := make([]CashFlowMonthDTO, len(r.Months))
	for i, m := range r.Months {
		months[i] = CashFlowMonthDTO{
			From: m.Period.From().String(), To: m.Period.To().String(),
			Inflow: m.Inflow.Minor(), Outflow: m.Outflow.Minor(), Net: m.Net.Minor(),
		}
	}
	return CashFlowDTO{
		From: r.Period.From().String(), To: r.Period.To().String(), Currency: r.Currency.Code(),
		Months: months, TotalInflow: r.TotalInflow.Minor(), TotalOutflow: r.TotalOutflow.Minor(),
		TotalNet: r.TotalNet.Minor(),
	}
}

func cashFlowRows(r reporting.CashFlowResult, loc domain.Locale) Rows {
	t := Rows{Header: []string{"MONTH", "INFLOW", "OUTFLOW", "NET"}, MoneyCols: []int{1, 2, 3}}
	for _, m := range r.Months {
		t.Body = append(t.Body, []string{
			FormatDate(m.Period.From(), loc), FormatMoney(m.Inflow, loc),
			FormatMoney(m.Outflow, loc), FormatMoney(m.Net, loc),
		})
	}
	t.Body = append(t.Body, []string{
		"TOTAL", FormatMoney(r.TotalInflow, loc), FormatMoney(r.TotalOutflow, loc), FormatMoney(r.TotalNet, loc),
	})
	return t
}

// WriteCashFlow renders the monthly cash-flow report.
func WriteCashFlow(w io.Writer, f Format, loc domain.Locale, color bool, r reporting.CashFlowResult) error {
	if f == JSON {
		return WriteJSON(w, cashFlowDTO(r))
	}
	if f == CSV {
		return WriteCSV(w, cashFlowRows(r, loc))
	}
	return WriteTable(w, cashFlowRows(r, loc), color)
}

// RunwayDTO is the --json shape of the runway KPI.
type RunwayDTO struct {
	AsOf                     string `json:"as_of"`
	Currency                 string `json:"currency"`
	Liquidity                int64  `json:"liquidity"`
	ReferenceMonthlySpending int64  `json:"reference_monthly_spending"`
	Months                   int    `json:"months"`
	Days                     int    `json:"days"`
	ZeroDate                 string `json:"zero_date"`
}

func runwayDTO(r reporting.RunwayResult) RunwayDTO {
	return RunwayDTO{
		AsOf: r.AsOf.String(), Currency: r.Currency.Code(), Liquidity: r.Liquidity.Minor(),
		ReferenceMonthlySpending: r.ReferenceMonthlySpending.Minor(), Months: r.Months,
		Days: r.Days, ZeroDate: r.ZeroDate.String(),
	}
}

func runwayRows(r reporting.RunwayResult, loc domain.Locale) Rows {
	return Rows{
		Header:    []string{"AS OF", "LIQUIDITY", "MONTHLY SPENDING", "MONTHS", "DAYS", "ZERO DATE"},
		MoneyCols: []int{1, 2},
		Body: [][]string{{
			FormatDate(r.AsOf, loc), FormatMoney(r.Liquidity, loc), FormatMoney(r.ReferenceMonthlySpending, loc),
			strconv.Itoa(r.Months), strconv.Itoa(r.Days), FormatDate(r.ZeroDate, loc),
		}},
	}
}

// WriteRunway renders the runway KPI.
func WriteRunway(w io.Writer, f Format, loc domain.Locale, color bool, r reporting.RunwayResult) error {
	if f == JSON {
		return WriteJSON(w, runwayDTO(r))
	}
	if f == CSV {
		return WriteCSV(w, runwayRows(r, loc))
	}
	return WriteTable(w, runwayRows(r, loc), color)
}
