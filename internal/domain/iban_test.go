package domain

import (
	"errors"
	"testing"
)

func TestParseIBAN(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr error
	}{
		{"german", "DE89 3704 0044 0532 0130 00", "DE89370400440532013000", nil},
		{"uk", "gb29nwbk60161331926819", "GB29NWBK60161331926819", nil},
		{"french with letter", "FR14 2004 1010 0505 0001 3M02 606", "FR1420041010050500013M02606", nil},
		{"empty", "", "", ErrMissingField},
		{"too short", "DE44", "", ErrInvalidValue},
		{"bad checksum", "DE89370400440532013001", "", ErrInvalidValue},
		{"lowercase country code digit", "D189370400440532013000", "", ErrInvalidValue},
		{"non alphanumeric", "DE89-704004-0440532013", "", ErrInvalidValue},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseIBAN(tt.in)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ParseIBAN(%q) err = %v, want %v", tt.in, err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Fatalf("ParseIBAN(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
