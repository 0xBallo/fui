package render

import (
	"io"

	"github.com/0xballo/fui/internal/domain"
)

// SplitRowView is one split row with its category already resolved to a name, ready to render.
type SplitRowView struct {
	Category string
	Amount   domain.Money
	Note     string
}

// SplitRowDTO is the --json shape of one split row.
type SplitRowDTO struct {
	Category string `json:"category"`
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
	Note     string `json:"note,omitempty"`
}

func splitRowDTO(v SplitRowView) SplitRowDTO {
	return SplitRowDTO{Category: v.Category, Amount: v.Amount.Minor(), Currency: v.Amount.Currency().Code(), Note: v.Note}
}

func splitTableRows(rows []SplitRowView, loc domain.Locale) Rows {
	t := Rows{Header: []string{"CATEGORY", "AMOUNT", "NOTE"}, MoneyCols: []int{1}}
	for _, v := range rows {
		t.Body = append(t.Body, []string{v.Category, FormatMoney(v.Amount, loc), v.Note})
	}
	return t
}

func splitCSVRows(rows []SplitRowView) Rows {
	t := Rows{Header: []string{"CATEGORY", "AMOUNT", "NOTE"}, MoneyCols: []int{1}}
	for _, v := range rows {
		t.Body = append(t.Body, []string{v.Category, FormatMoneyMachine(v.Amount), v.Note})
	}
	return t
}

// WriteSplitList renders the split rows of a transaction; an empty slice means it is not split.
func WriteSplitList(w io.Writer, f Format, loc domain.Locale, color bool, rows []SplitRowView) error {
	if f == JSON {
		dtos := make([]SplitRowDTO, len(rows))
		for i, v := range rows {
			dtos[i] = splitRowDTO(v)
		}
		return WriteJSON(w, dtos)
	}
	if f == CSV {
		return WriteCSV(w, splitCSVRows(rows))
	}
	return WriteTable(w, splitTableRows(rows, loc), color)
}
