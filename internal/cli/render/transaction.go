package render

import (
	"fmt"
	"io"
	"slices"

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

// TxTotalDTO is one currency's summed amount in a TxSummaryDTO, in minor units like
// TransactionDTO.Amount.
type TxTotalDTO struct {
	Currency string `json:"currency"`
	Amount   int64  `json:"amount"`
}

// TxSummaryDTO is the row count and the per-currency sum of a transaction page, per the technical
// spec's "Ricerca e filtri": the table ends with these, and --json carries them next to the rows.
type TxSummaryDTO struct {
	Count  int          `json:"count"`
	Totals []TxTotalDTO `json:"totals"`
}

// TxCursorDTO is the --json shape of a pagination cursor.
type TxCursorDTO struct {
	Date string `json:"date"`
	ID   int64  `json:"id"`
}

// TxPageDTO is the --json shape of "tx list" and "tx search": the page's rows, the summary and
// the cursor for the next page (null on the last page).
type TxPageDTO struct {
	Items   []TransactionDTO `json:"items"`
	Summary TxSummaryDTO     `json:"summary"`
	Next    *TxCursorDTO     `json:"next"`
}

// txTotals sums items.Amount per currency, in currency-code order (deterministic, independent of
// map iteration), and reports the row count alongside.
func txTotals(items []app.TxView) (count int, totals []domain.Money, err error) {
	var codes []string
	sums := map[string]domain.Money{}
	for _, v := range items {
		m := v.Transaction.Amount
		code := m.Currency().Code()
		cur, ok := sums[code]
		if !ok {
			codes = append(codes, code)
			sums[code] = m
			continue
		}
		sum, err := cur.Add(m)
		if err != nil {
			return 0, nil, fmt.Errorf("sum %s totals: %w", code, err)
		}
		sums[code] = sum
	}
	slices.Sort(codes)
	totals = make([]domain.Money, len(codes))
	for i, code := range codes {
		totals[i] = sums[code]
	}
	return len(items), totals, nil
}

func txSummaryDTO(count int, totals []domain.Money) TxSummaryDTO {
	dtos := make([]TxTotalDTO, len(totals))
	for i, m := range totals {
		dtos[i] = TxTotalDTO{Currency: m.Currency().Code(), Amount: m.Minor()}
	}
	return TxSummaryDTO{Count: count, Totals: dtos}
}

func txCursorDTO(c *app.TxCursor) *TxCursorDTO {
	if c == nil {
		return nil
	}
	return &TxCursorDTO{Date: c.Date.String(), ID: int64(c.ID)}
}

// writeTxSummaryLine prints the table-format summary: the row count and one "total" per
// currency, each formatted per locale like the rows above it.
func writeTxSummaryLine(w io.Writer, count int, totals []domain.Money, loc domain.Locale) error {
	noun := "transactions"
	if count == 1 {
		noun = "transaction"
	}
	line := fmt.Sprintf("%d %s", count, noun)
	for _, m := range totals {
		line += fmt.Sprintf(", total %s %s", m.Currency().Code(), FormatMoney(m, loc))
	}
	if _, err := fmt.Fprintln(w, line); err != nil {
		return fmt.Errorf("write tx summary: %w", err)
	}
	return nil
}

// WriteTxPage renders one page of a transaction listing with its summary: a table ends with the
// row count and the sum of amounts per currency; --json carries the same summary next to the
// items and the pagination cursor. --csv is the plain rows, unchanged, since a summary line would
// not be valid CSV data.
func WriteTxPage(w io.Writer, f Format, loc domain.Locale, color bool, page app.TxPage) error {
	count, totals, err := txTotals(page.Items)
	if err != nil {
		return err
	}
	if f == JSON {
		dtos := make([]TransactionDTO, len(page.Items))
		for i, v := range page.Items {
			dtos[i] = transactionDTO(v)
		}
		return WriteJSON(w, TxPageDTO{Items: dtos, Summary: txSummaryDTO(count, totals), Next: txCursorDTO(page.Next)})
	}
	if f == CSV {
		return WriteCSV(w, transactionCSVRows(page.Items))
	}
	if err := WriteTable(w, transactionTableRows(page.Items, loc), color); err != nil {
		return err
	}
	return writeTxSummaryLine(w, count, totals, loc)
}
