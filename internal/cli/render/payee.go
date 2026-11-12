package render

import (
	"io"
	"strings"

	"github.com/0xballo/fui/internal/domain"
)

// PayeeDTO is the --json shape of one payee.
type PayeeDTO struct {
	ID      int64    `json:"id"`
	Name    string   `json:"name"`
	Aliases []string `json:"aliases,omitempty"`
}

func payeeDTO(p domain.Payee) PayeeDTO {
	return PayeeDTO{ID: int64(p.ID), Name: p.Name, Aliases: p.Aliases}
}

func payeeRows(payees []domain.Payee) Rows {
	t := Rows{Header: []string{"NAME", "ALIASES"}}
	for _, p := range payees {
		t.Body = append(t.Body, []string{p.Name, strings.Join(p.Aliases, ",")})
	}
	return t
}

// WritePayeeList renders a list of payees.
func WritePayeeList(w io.Writer, f Format, payees []domain.Payee) error {
	if f == JSON {
		dtos := make([]PayeeDTO, len(payees))
		for i, p := range payees {
			dtos[i] = payeeDTO(p)
		}
		return WriteJSON(w, dtos)
	}
	if f == CSV {
		return WriteCSV(w, payeeRows(payees))
	}
	return WriteTable(w, payeeRows(payees), false)
}

// WritePayee renders one payee.
func WritePayee(w io.Writer, f Format, p domain.Payee) error {
	if f == JSON {
		return WriteJSON(w, payeeDTO(p))
	}
	return WritePayeeList(w, f, []domain.Payee{p})
}
