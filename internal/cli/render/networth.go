package render

import (
	"io"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

// NetWorthLineDTO is the --json shape of one account's contribution to net worth.
type NetWorthLineDTO struct {
	Name      string `json:"name"`
	Currency  string `json:"currency"`
	Balance   int64  `json:"balance"`
	Converted int64  `json:"converted"`
}

// NetWorthDTO is the --json shape of the net worth KPI.
type NetWorthDTO struct {
	AsOf     string            `json:"as_of"`
	Currency string            `json:"currency"`
	Total    int64             `json:"total"`
	Lines    []NetWorthLineDTO `json:"lines"`
}

func netWorthDTO(nw app.NetWorth) NetWorthDTO {
	lines := make([]NetWorthLineDTO, len(nw.Lines))
	for i, l := range nw.Lines {
		lines[i] = NetWorthLineDTO{
			Name: l.Account.Name, Currency: l.Account.Currency.Code(),
			Balance: l.Balance.Minor(), Converted: l.Converted.Minor(),
		}
	}
	return NetWorthDTO{AsOf: nw.At.String(), Currency: nw.Base.Code(), Total: nw.Total.Minor(), Lines: lines}
}

func netWorthTableRows(nw app.NetWorth, loc domain.Locale) Rows {
	t := Rows{Header: []string{"ACCOUNT", "CURRENCY", "BALANCE", "CONVERTED"}, MoneyCols: []int{2, 3}}
	for _, l := range nw.Lines {
		t.Body = append(t.Body, []string{
			l.Account.Name, l.Account.Currency.Code(), FormatMoney(l.Balance, loc), FormatMoney(l.Converted, loc),
		})
	}
	t.Body = append(t.Body, []string{"TOTAL", nw.Base.Code(), "", FormatMoney(nw.Total, loc)})
	return t
}

// netWorthCSVRows builds --csv rows: balances are always machine format, independent of locale
// (there is no loc parameter to misuse).
func netWorthCSVRows(nw app.NetWorth) Rows {
	t := Rows{Header: []string{"ACCOUNT", "CURRENCY", "BALANCE", "CONVERTED"}, MoneyCols: []int{2, 3}}
	for _, l := range nw.Lines {
		t.Body = append(t.Body, []string{
			l.Account.Name, l.Account.Currency.Code(), FormatMoneyMachine(l.Balance), FormatMoneyMachine(l.Converted),
		})
	}
	t.Body = append(t.Body, []string{"TOTAL", nw.Base.Code(), "", FormatMoneyMachine(nw.Total)})
	return t
}

// WriteNetWorth renders the net worth KPI: one line per account converted to the base currency,
// plus the total.
func WriteNetWorth(w io.Writer, f Format, loc domain.Locale, color bool, nw app.NetWorth) error {
	if f == JSON {
		return WriteJSON(w, netWorthDTO(nw))
	}
	if f == CSV {
		return WriteCSV(w, netWorthCSVRows(nw))
	}
	return WriteTable(w, netWorthTableRows(nw, loc), color)
}
