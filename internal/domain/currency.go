package domain

import (
	"strings"
)

// Currency is an ISO 4217 currency with its minor-unit decimals. It can only
// be obtained from ParseCurrency (known table), so a non-zero Currency is
// always valid. The zero value means "no currency".
type Currency struct {
	code     string
	decimals int
}

// currencyDecimals is the known-currency table: ISO 4217 code → minor-unit decimals.
func currencyDecimals(code string) (int, bool) {
	switch code {
	case "JPY":
		return 0, true
	case "EUR", "USD", "GBP", "CHF", "SEK", "NOK", "DKK", "PLN", "CZK", "HUF", "CAD", "AUD", "CNY":
		return 2, true
	}
	return 0, false
}

// ParseCurrency returns the Currency for an ISO 4217 code (case-insensitive).
func ParseCurrency(code string) (Currency, error) {
	c := strings.ToUpper(strings.TrimSpace(code))
	if c == "" {
		return Currency{}, missingField("currency")
	}
	d, ok := currencyDecimals(c)
	if !ok {
		return Currency{}, invalidValue("currency", code)
	}
	return Currency{code: c, decimals: d}, nil
}

// Code returns the ISO 4217 code, or "" for the zero Currency.
func (c Currency) Code() string { return c.code }

// Decimals returns the number of minor-unit decimals (EUR 2, JPY 0).
func (c Currency) Decimals() int { return c.decimals }

// IsZero reports whether c is the zero Currency.
func (c Currency) IsZero() bool { return c == Currency{} }

// String returns the ISO code.
func (c Currency) String() string { return c.code }
