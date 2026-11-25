package render

import (
	"fmt"
	"io"

	"github.com/0xballo/fui/internal/app/investing"
	"github.com/0xballo/fui/internal/domain"
)

// OpLineDTO is the --json shape of one listed security operation. UnitPrice is in minor units of
// PriceScale decimals.
type OpLineDTO struct {
	ID         int64  `json:"id"`
	Date       string `json:"date"`
	Kind       string `json:"kind"`
	Account    string `json:"account"`
	AccountID  int64  `json:"account_id"`
	Security   string `json:"security"`
	SecurityID int64  `json:"security_id"`
	Quantity   string `json:"quantity"`
	UnitPrice  int64  `json:"unit_price"`
	PriceScale int    `json:"price_scale"`
	Fees       int64  `json:"fees"`
	Currency   string `json:"currency"`
	CashTxID   int64  `json:"cash_tx_id,omitempty"`
}

func opLineDTO(l investing.OpLine) OpLineDTO {
	op := l.Op
	return OpLineDTO{
		ID: int64(op.ID), Date: op.Date.String(), Kind: string(op.Kind), Account: l.Account.Name, AccountID: int64(op.Account),
		Security: l.Security.Name, SecurityID: int64(op.Security), Quantity: op.Quantity.String(),
		UnitPrice: op.UnitPrice.Minor(), PriceScale: op.UnitPrice.Scale(), Fees: op.Fees.Minor(),
		Currency: op.UnitPrice.Currency().Code(), CashTxID: int64(op.CashTx),
	}
}

func opLineRows(lines []investing.OpLine, loc domain.Locale, machine bool) Rows {
	t := Rows{
		Header:    []string{"ID", "DATE", "KIND", "ACCOUNT", "SECURITY", "QUANTITY", "UNIT_PRICE", "FEES", "CURRENCY", "CASH_TX_ID"},
		MoneyCols: []int{6, 7},
	}
	for _, l := range lines {
		op := l.Op
		date, price, fees := FormatDate(op.Date, loc), FormatPrice(op.UnitPrice, loc), FormatMoney(op.Fees, loc)
		if machine {
			date, price, fees = FormatDateMachine(op.Date), op.UnitPrice.String(), FormatMoneyMachine(op.Fees)
		}
		cash := ""
		if op.CashTx != 0 {
			cash = fmt.Sprint(op.CashTx)
		} else if !machine {
			cash = "-"
		}
		t.Body = append(t.Body, []string{
			fmt.Sprint(op.ID), date, string(op.Kind), l.Account.Name, l.Security.Name, op.Quantity.String(),
			price, fees, op.UnitPrice.Currency().Code(), cash,
		})
	}
	return t
}

// WriteOpList renders listed security operations.
func WriteOpList(w io.Writer, f Format, loc domain.Locale, color bool, lines []investing.OpLine) error {
	if f == JSON {
		dtos := make([]OpLineDTO, len(lines))
		for i, l := range lines {
			dtos[i] = opLineDTO(l)
		}
		return WriteJSON(w, dtos)
	}
	if f == CSV {
		return WriteCSV(w, opLineRows(lines, loc, true))
	}
	return WriteTable(w, opLineRows(lines, loc, false), color)
}

// SecurityPositionDTO is the --json shape of one security aggregated over every account, valued
// in the base currency.
type SecurityPositionDTO struct {
	Security   string `json:"security"`
	SecurityID int64  `json:"security_id"`
	Kind       string `json:"kind"`
	Quantity   string `json:"quantity"`
	Accounts   int    `json:"accounts"`
	Currency   string `json:"currency"`
	CostBasis  int64  `json:"cost_basis"`
	HasPrice   bool   `json:"has_price"`
	Value      int64  `json:"value,omitempty"`
	Unrealized int64  `json:"unrealized,omitempty"`
}

func securityPositionDTO(l investing.SecurityPositionLine) SecurityPositionDTO {
	dto := SecurityPositionDTO{
		Security: l.Security.Name, SecurityID: int64(l.Security.ID), Kind: string(l.Security.Kind),
		Quantity: l.Quantity.String(), Accounts: l.Accounts, Currency: l.CostBasis.Currency().Code(),
		CostBasis: l.CostBasis.Minor(), HasPrice: l.HasPrice,
	}
	if l.HasPrice {
		dto.Value, dto.Unrealized = l.Value.Minor(), l.Unrealized.Minor()
	}
	return dto
}

func securityPositionRows(lines []investing.SecurityPositionLine, loc domain.Locale, machine bool) Rows {
	t := Rows{
		Header:    []string{"SECURITY", "KIND", "QUANTITY", "ACCOUNTS", "CURRENCY", "COST_BASIS", "VALUE", "UNREALIZED"},
		MoneyCols: []int{5, 6, 7},
	}
	for _, l := range lines {
		cost, value, unreal := FormatMoney(l.CostBasis, loc), "-", "-"
		if machine {
			cost, value, unreal = FormatMoneyMachine(l.CostBasis), "", ""
		}
		if l.HasPrice {
			if machine {
				value, unreal = FormatMoneyMachine(l.Value), FormatMoneyMachine(l.Unrealized)
			} else {
				value, unreal = FormatMoney(l.Value, loc), FormatMoney(l.Unrealized, loc)
			}
		}
		t.Body = append(t.Body, []string{
			l.Security.Name, string(l.Security.Kind), l.Quantity.String(), fmt.Sprint(l.Accounts),
			l.CostBasis.Currency().Code(), cost, value, unreal,
		})
	}
	return t
}

// WriteSecurityPositionList renders securities aggregated over every account.
func WriteSecurityPositionList(w io.Writer, f Format, loc domain.Locale, color bool, lines []investing.SecurityPositionLine) error {
	if f == JSON {
		dtos := make([]SecurityPositionDTO, len(lines))
		for i, l := range lines {
			dtos[i] = securityPositionDTO(l)
		}
		return WriteJSON(w, dtos)
	}
	if f == CSV {
		return WriteCSV(w, securityPositionRows(lines, loc, true))
	}
	return WriteTable(w, securityPositionRows(lines, loc, false), color)
}
