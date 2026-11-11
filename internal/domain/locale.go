package domain

import (
	"strings"
)

// Locale selects number and date conventions for parsing and display.
type Locale string

// Supported locales.
const (
	LocaleITIT Locale = "it-IT" // 1.234,56 · DD/MM/YYYY
	LocaleENUS Locale = "en-US" // 1,234.56 · MM/DD/YYYY
)

// ParseLocale returns the supported Locale named s (case-insensitive).
func ParseLocale(s string) (Locale, error) {
	for _, l := range []Locale{LocaleITIT, LocaleENUS} {
		if strings.EqualFold(strings.TrimSpace(s), string(l)) {
			return l, nil
		}
	}
	return "", invalidValue("locale", s)
}

// Valid reports whether l is a supported locale.
func (l Locale) Valid() bool { return l == LocaleITIT || l == LocaleENUS }

// separators returns the decimal and thousands separators; unsupported
// locales fall back to en-US so formatting never fails.
func (l Locale) separators() (decimal, group string) {
	switch l {
	case LocaleITIT:
		return ",", "."
	case LocaleENUS:
		return ".", ","
	}
	return ".", ","
}
