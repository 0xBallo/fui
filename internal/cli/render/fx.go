package render

import (
	"io"

	"github.com/0xballo/fui/internal/domain"
)

// FXRateDTO is the --json shape of one manual exchange rate.
type FXRateDTO struct {
	Currency string `json:"currency"`
	Date     string `json:"date"`
	Rate     string `json:"rate"`
}

func fxRateDTO(r domain.FXRate) FXRateDTO {
	return FXRateDTO{Currency: r.Currency.Code(), Date: r.Date.String(), Rate: FormatRate(r.Rate)}
}

func fxRateRows(rates []domain.FXRate) Rows {
	t := Rows{Header: []string{"CURRENCY", "DATE", "RATE"}}
	for _, r := range rates {
		t.Body = append(t.Body, []string{r.Currency.Code(), r.Date.String(), FormatRate(r.Rate)})
	}
	return t
}

// WriteFXRateList renders a list of manual exchange rates.
func WriteFXRateList(w io.Writer, f Format, rates []domain.FXRate) error {
	if f == JSON {
		dtos := make([]FXRateDTO, len(rates))
		for i, r := range rates {
			dtos[i] = fxRateDTO(r)
		}
		return WriteJSON(w, dtos)
	}
	if f == CSV {
		return WriteCSV(w, fxRateRows(rates))
	}
	return WriteTable(w, fxRateRows(rates), false)
}

// WriteFXRate renders one exchange rate.
func WriteFXRate(w io.Writer, f Format, r domain.FXRate) error {
	if f == JSON {
		return WriteJSON(w, fxRateDTO(r))
	}
	return WriteFXRateList(w, f, []domain.FXRate{r})
}
