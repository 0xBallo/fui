package components

import (
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"

	"github.com/0xballo/fui/internal/tui/theme"
)

func lineSeries(name string, vals ...int64) Series {
	pts := make([]Point, len(vals))
	for i, v := range vals {
		pts[i] = Point{Label: "2026-" + strconv.Itoa(i+1), Value: v}
	}
	return Series{Name: name, Points: pts}
}

func lineChart(t *testing.T, s ...Series) LineChart {
	t.Helper()
	c := NewLineChart(theme.Resolve(theme.Config{Theme: "mocha"}, nil))
	c.SetSize(60, 12)
	c.SetSeries(s...)
	return c
}

func TestLineChartGolden(t *testing.T) {
	a := lineSeries("Income", 100000, 120000, 90000, 150000, 130000)
	b := lineSeries("Expenses", 80000, 85000, 95000, 70000, 110000)
	neg := lineSeries("Net", -5000, 2000, -8000, 4000)
	tests := []struct {
		name   string
		series []Series
	}{
		{"One", []Series{a}},
		{"Two", []Series{a, b}},
		{"Negative", []Series{neg}},
		{"Empty", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			golden.RequireEqual(t, []byte(ansi.Strip(lineChart(t, tc.series...).View())))
		})
	}
}

func TestLineChartDeterministicAndLegend(t *testing.T) {
	c := lineChart(t, lineSeries("A", 1, 5, 3), lineSeries("B", 2, 2, 2))
	c.SetYFormatter(func(v int64) string { return "€" + strconv.FormatInt(v, 10) })
	first := c.View()
	if first != c.View() {
		t.Fatal("View is not deterministic")
	}
	out := ansi.Strip(first)
	if !strings.Contains(out, "€") {
		t.Errorf("Y formatter not applied:\n%s", out)
	}
	last := out[strings.LastIndex(out, "\n")+1:]
	if !strings.Contains(last, "A") || !strings.Contains(last, "B") {
		t.Errorf("legend missing names: %q", last)
	}
}

func TestLineChartFlatAndExtraSeries(t *testing.T) {
	var many []Series
	for range 6 {
		many = append(many, lineSeries("S", 7, 7))
	}
	c := lineChart(t, many...)
	if got := len(c.series); got != 4 {
		t.Fatalf("series kept = %d, want 4", got)
	}
	if c.View() == "" {
		t.Fatal("empty view for flat series")
	}
}
