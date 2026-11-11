package render

import (
	"io"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

// AccountDTO is the --json shape of one account with its balance.
type AccountDTO struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description,omitempty"`
	Kind           string `json:"kind"`
	Currency       string `json:"currency"`
	OpeningBalance int64  `json:"opening_balance"`
	OpenedOn       string `json:"opened_on"`
	Archived       bool   `json:"archived"`
	IBAN           string `json:"iban,omitempty"`
	Balance        int64  `json:"balance"`
	AsOf           string `json:"as_of"`
}

func accountDTO(a app.AccountBalance) AccountDTO {
	return AccountDTO{
		ID: int64(a.Account.ID), Name: a.Account.Name, Description: a.Account.Description,
		Kind: string(a.Account.Kind), Currency: a.Account.Currency.Code(),
		OpeningBalance: a.Account.OpeningBalance.Minor(), OpenedOn: a.Account.OpenedOn.String(),
		Archived: a.Account.Archived, IBAN: a.Account.IBAN, Balance: a.Balance.Minor(), AsOf: a.At.String(),
	}
}

func accountRows(accts []app.AccountBalance, loc domain.Locale) Rows {
	t := Rows{Header: []string{"NAME", "KIND", "CURRENCY", "BALANCE", "ARCHIVED"}, MoneyCols: []int{3}}
	for _, a := range accts {
		t.Body = append(t.Body, []string{
			a.Account.Name, string(a.Account.Kind), a.Account.Currency.Code(),
			FormatMoney(a.Balance, loc), boolCell(a.Account.Archived),
		})
	}
	return t
}

func boolCell(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// WriteAccountList renders a list of accounts with their balance.
func WriteAccountList(w io.Writer, f Format, loc domain.Locale, color bool, accts []app.AccountBalance) error {
	if f == JSON {
		dtos := make([]AccountDTO, len(accts))
		for i, a := range accts {
			dtos[i] = accountDTO(a)
		}
		return WriteJSON(w, dtos)
	}
	if f == CSV {
		return WriteCSV(w, accountRows(accts, loc))
	}
	return WriteTable(w, accountRows(accts, loc), color)
}

// WriteAccount renders one account with its balance.
func WriteAccount(w io.Writer, f Format, loc domain.Locale, color bool, a app.AccountBalance) error {
	if f == JSON {
		return WriteJSON(w, accountDTO(a))
	}
	return WriteAccountList(w, f, loc, color, []app.AccountBalance{a})
}
