package domain

// FXRate is a manually entered exchange rate: on Date, one unit of Currency
// is worth Rate units of the user's base currency.
type FXRate struct {
	Currency Currency
	Date     Date
	Rate     Rate
}

// NewFXRate validates and returns an FXRate.
func NewFXRate(c Currency, d Date, r Rate) (FXRate, error) {
	if c.IsZero() {
		return FXRate{}, missingField("currency")
	}
	if d.IsZero() {
		return FXRate{}, missingField("date")
	}
	if _, err := NewRate(r.Scaled()); err != nil {
		return FXRate{}, err
	}
	return FXRate{Currency: c, Date: d, Rate: r}, nil
}
