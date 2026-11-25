package render

import (
	"strings"
	"testing"

	"github.com/0xballo/fui/internal/domain"
)

func TestWriteTableAlignsAccentedCells(t *testing.T) {
	var b strings.Builder
	rows := Rows{Header: []string{"DESCRIPTION", "AMOUNT"}, Body: [][]string{{"Caffè Città", "-12,50"}, {"Bar", "-1,00"}}}
	if err := WriteTable(&b, rows, false); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(b.String(), "\n"), "\n")
	want := strings.Index(lines[0], "AMOUNT")
	for _, l := range lines[1:] {
		if got := strings.IndexRune(l, '-'); len([]rune(l[:got])) != want {
			t.Errorf("amount column of %q starts at rune %d, want %d", l, len([]rune(l[:got])), want)
		}
	}
}

func TestWriteRuleListShowsHintConditions(t *testing.T) {
	r := domain.Rule{
		Name: "hinted", Priority: 1, Enabled: true,
		Conditions: domain.RuleConditions{HintContains: "gro", HintEquals: "groceries"},
		Actions:    domain.RuleActions{CategoryID: 1},
	}
	lookup := NewRuleLookup(nil, []domain.Category{{ID: 1, Name: "Food"}}, nil, nil)

	var human strings.Builder
	if err := WriteRuleList(&human, Table, []domain.Rule{r}, lookup); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(human.String(), "hint-contains=gro hint-equals=groceries") && !strings.Contains(human.String(), "hint-contains=gro") {
		t.Errorf("table output lacks hint conditions:\n%s", human.String())
	}

	var js strings.Builder
	if err := WriteRule(&js, JSON, r, lookup); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"hint_contains": "gro"`, `"hint_equals": "groceries"`} {
		if !strings.Contains(strings.ReplaceAll(js.String(), `":"`, `": "`), want) {
			t.Errorf("json output lacks %s:\n%s", want, js.String())
		}
	}
}

func TestFormatPrice(t *testing.T) {
	eur, err := domain.ParseCurrency("EUR")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		minor int64
		scale int
		loc   domain.Locale
		want  string
	}{
		{1234, 8, domain.LocaleENUS, "0.00001234"},
		{1234, 8, domain.LocaleITIT, "0,00001234"},
		{123456789012, 4, domain.LocaleITIT, "12.345.678,9012"},
		{123456789012, 4, domain.LocaleENUS, "12,345,678.9012"},
		{-50, 2, domain.LocaleENUS, "-0.50"},
		{700, 2, domain.LocaleITIT, "7,00"},
	}
	for _, tt := range tests {
		p, err := domain.NewPrice(tt.minor, tt.scale, eur)
		if err != nil {
			t.Fatal(err)
		}
		if got := FormatPrice(p, tt.loc); got != tt.want {
			t.Errorf("FormatPrice(%d,%d,%s) = %q, want %q", tt.minor, tt.scale, tt.loc, got, tt.want)
		}
	}
}
