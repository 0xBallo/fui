package main

import "testing"

func TestParseEuroMinorUnits(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  int64
	}{
		{name: "integer", value: "42", want: 4200},
		{name: "comma decimals", value: "-12,34", want: -1234},
		{name: "dot decimals", value: "1.2", want: 120},
		{name: "thousands and commas", value: "1.234,56", want: 123456},
		{name: "round half away from zero", value: "-1,235", want: -124},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseEuroMinorUnits(tc.value)
			if err != nil {
				t.Fatalf("parseEuroMinorUnits returned error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("parseEuroMinorUnits(%q) = %d, want %d", tc.value, got, tc.want)
			}
		})
	}
}

func TestParseWidibaDate(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "iso", value: "2025-04-10", want: "2025-04-10"},
		{name: "slash", value: "10/04/2025", want: "2025-04-10"},
		{name: "excel serial", value: "45757", want: "2025-04-10"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseWidibaDate(tc.value)
			if err != nil {
				t.Fatalf("parseWidibaDate returned error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("parseWidibaDate(%q) = %q, want %q", tc.value, got, tc.want)
			}
		})
	}
}

func TestHelperCoverage(t *testing.T) {
	if got := fallbackLatestDate([]widibaMovement{
		{date: "2025-01-01"},
		{date: "2025-01-03"},
		{date: "2025-01-02"},
	}); got != "2025-01-03" {
		t.Fatalf("fallbackLatestDate = %q", got)
	}

	errRecord := widibaError(7, "bad row", map[string]string{"k": "v"})
	if errRecord.Line != 7 || errRecord.Message != "bad row" {
		t.Fatalf("unexpected widibaError: %+v", errRecord)
	}

	if suffix := accountSuffix("IT00ABCD"); suffix != "abcd" {
		t.Fatalf("accountSuffix fallback = %q", suffix)
	}

	if _, err := parseSerialInt("not-a-number"); err == nil {
		t.Fatalf("expected parseSerialInt error")
	}
}
