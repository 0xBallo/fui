package render

import (
	"io"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

// TransactionDTO is the --json shape of one transaction.
type TransactionDTO struct {
	ID          int64  `json:"id"`
	Date        string `json:"date"`
	Account     string `json:"account"`
	Category    string `json:"category,omitempty"`
	Description string `json:"description,omitempty"`
	Notes       string `json:"notes,omitempty"`
	Status      string `json:"status"`
	Kind        string `json:"kind"`
	Amount      int64  `json:"amount"`
	Currency    string `json:"currency"`
	TransferID  int64  `json:"transfer_id,omitempty"`
}

func transactionDTO(v app.TxView) TransactionDTO {
	return TransactionDTO{
		ID: int64(v.Transaction.ID), Date: v.Transaction.Date.String(), Account: v.Account,
		Category: v.Category, Description: v.Transaction.Description, Notes: v.Transaction.Notes,
		Status: string(v.Transaction.Status), Kind: string(v.Transaction.Kind),
		Amount: v.Transaction.Amount.Minor(), Currency: v.Transaction.Amount.Currency().Code(),
		TransferID: int64(v.Transaction.TransferGroupID),
	}
}

func transactionTableRows(items []app.TxView, loc domain.Locale) Rows {
	t := Rows{Header: []string{"DATE", "ACCOUNT", "CATEGORY", "DESCRIPTION", "AMOUNT", "STATUS"}, MoneyCols: []int{4}}
	for _, v := range items {
		t.Body = append(t.Body, []string{
			FormatDate(v.Transaction.Date, loc), v.Account, v.Category, v.Transaction.Description,
			FormatMoney(v.Transaction.Amount, loc), string(v.Transaction.Status),
		})
	}
	return t
}

// transactionCSVRows builds --csv rows: dates and amounts are always machine format, independent
// of locale (there is no loc parameter to misuse).
func transactionCSVRows(items []app.TxView) Rows {
	t := Rows{Header: []string{"DATE", "ACCOUNT", "CATEGORY", "DESCRIPTION", "AMOUNT", "STATUS"}, MoneyCols: []int{4}}
	for _, v := range items {
		t.Body = append(t.Body, []string{
			FormatDateMachine(v.Transaction.Date), v.Account, v.Category, v.Transaction.Description,
			FormatMoneyMachine(v.Transaction.Amount), string(v.Transaction.Status),
		})
	}
	return t
}

// WriteTxList renders a page of transactions.
func WriteTxList(w io.Writer, f Format, loc domain.Locale, color bool, items []app.TxView) error {
	if f == JSON {
		dtos := make([]TransactionDTO, len(items))
		for i, v := range items {
			dtos[i] = transactionDTO(v)
		}
		return WriteJSON(w, dtos)
	}
	if f == CSV {
		return WriteCSV(w, transactionCSVRows(items))
	}
	return WriteTable(w, transactionTableRows(items, loc), color)
}

// WriteTx renders one transaction.
func WriteTx(w io.Writer, f Format, loc domain.Locale, color bool, v app.TxView) error {
	if f == JSON {
		return WriteJSON(w, transactionDTO(v))
	}
	return WriteTxList(w, f, loc, color, []app.TxView{v})
}
