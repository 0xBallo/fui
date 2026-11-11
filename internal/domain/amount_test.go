package domain

import (
	"errors"
	"testing"

	"pgregory.net/rapid" //nolint:depguard // test-only dependency allowed by the technical spec
)

func TestParseAmount(t *testing.T) {
	tests := []struct {
		in      string
		loc     Locale
		cur     string
		want    int64
		wantErr error
	}{
		{"12,34", LocaleITIT, "EUR", 1234, nil},
		{"12.34", LocaleENUS, "EUR", 1234, nil},
		{"1.234,56", LocaleITIT, "EUR", 123456, nil},
		{"1,234.56", LocaleENUS, "EUR", 123456, nil},
		{"-1.234.567,5", LocaleITIT, "EUR", -123456750, nil},
		{"+7", LocaleENUS, "EUR", 700, nil},
		{"  42 ", LocaleENUS, "EUR", 4200, nil},
		{"1234", LocaleITIT, "EUR", 123400, nil},
		{"1.234", LocaleITIT, "EUR", 123400, nil}, // locale decides: dot is thousands in it-IT
		{"1.234", LocaleENUS, "EUR", 123, nil},    // ...and a decimal point in en-US (rounded)
		{"0,005", LocaleITIT, "EUR", 1, nil},      // half rounds away from zero
		{"-0,005", LocaleITIT, "EUR", -1, nil},
		{"0,004", LocaleITIT, "EUR", 0, nil},
		{"-0", LocaleENUS, "EUR", 0, nil},
		{"1234.5", LocaleENUS, "JPY", 1235, nil},
		{"1234", LocaleENUS, "JPY", 1234, nil},
		{"9223372036854775807", LocaleENUS, "JPY", 9223372036854775807, nil},
		{"", LocaleENUS, "EUR", 0, ErrInvalidAmount},
		{"-", LocaleENUS, "EUR", 0, ErrInvalidAmount},
		{"abc", LocaleENUS, "EUR", 0, ErrInvalidAmount},
		{"1,234.56", LocaleITIT, "EUR", 0, ErrInvalidAmount}, // wrong-locale separators
		{"1.234,56", LocaleENUS, "EUR", 0, ErrInvalidAmount}, // wrong-locale separators
		{"12,34,56", LocaleENUS, "EUR", 0, ErrInvalidAmount}, // bad grouping
		{"1,23", LocaleENUS, "EUR", 0, ErrInvalidAmount},     // group of 2
		{"1234,567", LocaleENUS, "EUR", 0, ErrInvalidAmount}, // first group too long
		{",5", LocaleITIT, "EUR", 0, ErrInvalidAmount},       // no integer part
		{"5,", LocaleITIT, "EUR", 0, ErrInvalidAmount},       // dangling decimal separator
		{"1 234", LocaleITIT, "EUR", 0, ErrInvalidAmount},    // inner space
		{"--1", LocaleENUS, "EUR", 0, ErrInvalidAmount},      // double sign
		{"١٢", LocaleENUS, "EUR", 0, ErrInvalidAmount},       // non-ASCII digits
		{"1e3", LocaleENUS, "EUR", 0, ErrInvalidAmount},      // exponent
		{"9223372036854775808", LocaleENUS, "JPY", 0, ErrOverflow},
		{"92233720368547759", LocaleENUS, "EUR", 0, ErrOverflow},
		{"11111111111111111111111111111111111111111111111111111111111111111", LocaleENUS, "EUR", 0, ErrInvalidAmount}, // too long
	}
	for _, tt := range tests {
		t.Run(tt.in+"/"+string(tt.loc), func(t *testing.T) {
			got, err := ParseAmount(tt.in, tt.loc, cur(t, tt.cur))
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil || got.Minor() != tt.want || got.Currency().Code() != tt.cur {
				t.Fatalf("got %v, %v; want %d %s", got, err, tt.want, tt.cur)
			}
		})
	}
}

func TestParseAmountBadArguments(t *testing.T) {
	if _, err := ParseAmount("1", "xx", cur(t, "EUR")); !errors.Is(err, ErrInvalidValue) {
		t.Errorf("bad locale err = %v", err)
	}
	if _, err := ParseAmount("1", LocaleENUS, Currency{}); !errors.Is(err, ErrMissingField) {
		t.Errorf("zero currency err = %v", err)
	}
}

func TestFormatAmount(t *testing.T) {
	tests := []struct {
		minor int64
		cur   string
		loc   Locale
		want  string
	}{
		{123456, "EUR", LocaleITIT, "1.234,56"},
		{123456, "EUR", LocaleENUS, "1,234.56"},
		{-5, "EUR", LocaleITIT, "-0,05"},
		{0, "EUR", LocaleENUS, "0.00"},
		{100000000, "USD", LocaleITIT, "1.000.000,00"},
		{999, "EUR", LocaleENUS, "9.99"},
		{-1234567, "JPY", LocaleENUS, "-1,234,567"},
		{5, "JPY", LocaleITIT, "5"},
		{-9223372036854775808, "JPY", LocaleENUS, "-9,223,372,036,854,775,808"},
	}
	for _, tt := range tests {
		if got := FormatAmount(money(t, tt.minor, tt.cur), tt.loc); got != tt.want {
			t.Errorf("FormatAmount(%d %s, %s) = %q, want %q", tt.minor, tt.cur, tt.loc, got, tt.want)
		}
	}
	if got := FormatMachine(money(t, -123456, "EUR")); got != "-1234.56" {
		t.Errorf("FormatMachine = %q", got)
	}
}

func TestAmountRoundTripProperty(t *testing.T) {
	codes := []string{"EUR", "USD", "JPY", "HUF", "CHF"}
	rapid.Check(t, func(t *rapid.T) {
		c := cur(t, rapid.SampledFrom(codes).Draw(t, "currency"))
		m, _ := NewMoney(rapid.Int64().Draw(t, "minor"), c)
		for _, loc := range []Locale{LocaleITIT, LocaleENUS} {
			got, err := ParseAmount(FormatAmount(m, loc), loc, c)
			if err != nil || got != m {
				t.Fatalf("%s round trip of %v = %v, %v", loc, m, got, err)
			}
		}
		got, err := ParseAmount(FormatMachine(m), LocaleENUS, c)
		if err != nil || got != m {
			t.Fatalf("machine round trip of %v = %v, %v", m, got, err)
		}
	})
}

func FuzzParseAmount(f *testing.F) {
	for _, s := range []string{"12,34", "1.234,56", "-0", "+1,234.5", "", ",", "1..2", "9223372036854775808", "٣"} {
		f.Add(s, true)
		f.Add(s, false)
	}
	eur, _ := ParseCurrency("EUR")
	f.Fuzz(func(t *testing.T, s string, italian bool) {
		loc := LocaleENUS
		if italian {
			loc = LocaleITIT
		}
		m, err := ParseAmount(s, loc, eur)
		if err != nil {
			var de *Error
			if !errors.As(err, &de) {
				t.Fatalf("non-domain error %T: %v", err, err)
			}
			return
		}
		back, err := ParseAmount(FormatAmount(m, loc), loc, eur)
		if err != nil || back != m {
			t.Fatalf("parse(%q) = %v but reparse of its format = %v, %v", s, m, back, err)
		}
	})
}
