package domain

import (
	"errors"
	"testing"
)

func TestParseCurrency(t *testing.T) {
	for _, code := range []string{"EUR", "USD", "GBP", "CHF", "JPY", "SEK", "NOK", "DKK", "PLN", "CZK", "HUF", "CAD", "AUD", "CNY"} {
		c, err := ParseCurrency(code)
		if err != nil || c.Code() != code {
			t.Errorf("ParseCurrency(%q) = %v, %v", code, c, err)
		}
	}
	tests := []struct {
		in       string
		code     string
		decimals int
		wantErr  error
	}{
		{in: "eur ", code: "EUR", decimals: 2},
		{in: "JPY", code: "JPY", decimals: 0},
		{in: "XXX", wantErr: ErrInvalidValue},
		{in: "", wantErr: ErrMissingField},
	}
	for _, tt := range tests {
		c, err := ParseCurrency(tt.in)
		if tt.wantErr != nil {
			if !errors.Is(err, tt.wantErr) || !c.IsZero() {
				t.Errorf("ParseCurrency(%q) = %v, %v; want %v", tt.in, c, err, tt.wantErr)
			}
			continue
		}
		if err != nil || c.Code() != tt.code || c.Decimals() != tt.decimals || c.String() != tt.code || c.IsZero() {
			t.Errorf("ParseCurrency(%q) = %+v, %v", tt.in, c, err)
		}
	}
}

func TestParseLocale(t *testing.T) {
	if l, err := ParseLocale(" IT-it "); err != nil || l != LocaleITIT {
		t.Errorf("ParseLocale = %v, %v", l, err)
	}
	if _, err := ParseLocale("fr-FR"); !errors.Is(err, ErrInvalidValue) {
		t.Errorf("err = %v", err)
	}
	if Locale("fr-FR").Valid() {
		t.Error("fr-FR must not be valid")
	}
	if d, g := Locale("fr-FR").separators(); d != "." || g != "," {
		t.Errorf("fallback separators = %q %q", d, g)
	}
}
