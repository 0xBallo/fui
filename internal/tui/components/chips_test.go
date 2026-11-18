package components_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/theme"
)

func TestChips(t *testing.T) {
	th := theme.Default()
	tests := []struct {
		name   string
		labels []string
		width  int
		want   string
	}{
		{"fits", []string{"a", "bb"}, 20, " a   bb "},
		{"cuts mid chip", []string{"food", "trip"}, 8, " food  …"},
		{"width one", []string{"a"}, 1, "…"},
		{"width zero", []string{"a"}, 0, ""},
		{"no labels", nil, 10, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ansi.Strip(components.Chips(th, tt.labels, tt.width))
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRowStyleAndBarStyle(t *testing.T) {
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI256)
	t.Cleanup(func() { lipgloss.SetColorProfile(prev) })
	red := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))

	tbl := components.NewTable([]components.Column{{Title: "A", Width: 6}}, theme.Default())
	tbl.SetRows([]components.Row{{ID: "1", Cells: []string{"plain"}}, {ID: "2", Cells: []string{"red"}, Style: red}})
	if v := tbl.View(); !containsSeq(v, "38;5;196") {
		t.Errorf("styled row not colored when cursor elsewhere:\n%q", v)
	}
	tbl.SetCursor(1)
	if v := tbl.View(); !containsSeq(v, "red") {
		t.Errorf("selected styled row missing:\n%q", v)
	}

	chart := components.NewBarChart(theme.Default())
	chart.SetItems([]components.BarItem{
		{Label: "in", Value: 100, DisplayValue: "+100"},
		{Label: "out", Value: -50, DisplayValue: "-50", Style: red},
	})
	v := chart.View()
	if !containsSeq(v, "38;5;196") {
		t.Errorf("custom bar style not applied:\n%q", v)
	}
	if n := countRune(ansi.Strip(v), '█'); n == 0 {
		t.Errorf("no bars rendered:\n%s", v)
	}
}

func containsSeq(s, sub string) bool { return strings.Contains(s, sub) }

func countRune(s string, r rune) int { return strings.Count(s, string(r)) }
