package render

import (
	"strings"
	"testing"
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
