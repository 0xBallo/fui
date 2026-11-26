package components

import (
	"image/color"
	"math"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/NimbleMarkets/ntcharts/v2/canvas"
	"github.com/NimbleMarkets/ntcharts/v2/linechart"

	"github.com/0xballo/fui/internal/tui/theme"
)

// maxSeries is the number of series a LineChart draws; extra ones are ignored.
const maxSeries = 4

// Point is one sample of a line series.
type Point struct {
	// Label is the X tick text (for example "2026-03").
	Label string
	// Value is in minor units.
	Value int64
}

// Series is a named sequence of points drawn as one line.
type Series struct {
	Name   string
	Points []Point
	// Style colors the line; the zero value falls back to the theme's Series1..4 by position.
	Style lipgloss.Style
}

// LineChart renders up to four series as a braille line plot with a legend line below it.
type LineChart struct {
	series  []Series
	width   int
	height  int
	theme   theme.Theme
	yFormat func(int64) string
}

// NewLineChart constructs a chart initialized with an active visual theme.
func NewLineChart(th theme.Theme) LineChart {
	return LineChart{theme: th, width: 60, height: 12}
}

// SetSize assigns the total character space, legend included.
func (c *LineChart) SetSize(w, h int) {
	if w > 0 {
		c.width = w
	}
	if h > 0 {
		c.height = h
	}
}

// SetSeries replaces the drawn series; only the first four are kept.
func (c *LineChart) SetSeries(s ...Series) {
	if len(s) > maxSeries {
		s = s[:maxSeries]
	}
	c.series = s
}

// SetYFormatter sets how Y axis values (minor units) are rendered; nil restores the plain integer.
func (c *LineChart) SetYFormatter(f func(int64) string) {
	c.yFormat = f
}

// View renders the plot and the legend, or a muted placeholder when no series has points.
func (c LineChart) View() string {
	n, lo, hi := c.extent()
	if n == 0 {
		return c.theme.Muted.Render("(no data)")
	}
	if lo == hi {
		lo--
		hi++
	}

	labels := c.xLabels(n)
	yfmt := c.yFormat
	if yfmt == nil {
		yfmt = func(v int64) string { return strconv.FormatInt(v, 10) }
	}
	axis := c.theme.Muted
	h := max(c.height-1, 3)
	m := linechart.New(c.width, h, 0, math.Max(float64(n-1), 1), float64(lo), float64(hi),
		linechart.WithStyles(axis, axis, lipgloss.NewStyle()),
		linechart.WithYLabelFormatter(func(_ int, v float64) string { return yfmt(int64(math.Round(v))) }),
		linechart.WithXLabelFormatter(func(_ int, v float64) string {
			i := int(math.Round(v))
			if i < 0 || i >= len(labels) {
				return ""
			}
			return labels[i]
		}),
	)
	m.DrawXYAxisAndLabel()
	for i, s := range c.series {
		drawSeries(&m, s.Points, c.style(i))
	}
	return m.View() + "\n" + c.legend()
}

// extent returns the longest series length and the min/max values across all series.
func (c LineChart) extent() (n int, lo, hi int64) {
	first := true
	for _, s := range c.series {
		n = max(n, len(s.Points))
		for _, p := range s.Points {
			if first || p.Value < lo {
				lo = p.Value
			}
			if first || p.Value > hi {
				hi = p.Value
			}
			first = false
		}
	}
	return n, lo, hi
}

// xLabels takes each tick label from the first series that has a point at that index.
func (c LineChart) xLabels(n int) []string {
	out := make([]string, n)
	for i := range out {
		for _, s := range c.series {
			if i < len(s.Points) {
				out[i] = s.Points[i].Label
				break
			}
		}
	}
	return out
}

func (c LineChart) style(i int) lipgloss.Style {
	if s := c.series[i].Style; s.GetForeground() != (lipgloss.NoColor{}) {
		return s
	}
	tk := c.theme.Tokens
	cols := [maxSeries]color.Color{tk.Series1, tk.Series2, tk.Series3, tk.Series4}
	return lipgloss.NewStyle().Foreground(cols[i])
}

func (c LineChart) legend() string {
	parts := make([]string, 0, len(c.series))
	for i, s := range c.series {
		parts = append(parts, c.style(i).Render("●")+" "+c.theme.Muted.Render(s.Name))
	}
	return strings.Join(parts, "  ")
}

func drawSeries(m *linechart.Model, pts []Point, st lipgloss.Style) {
	for i := range pts {
		cur := canvas.Float64Point{X: float64(i), Y: float64(pts[i].Value)}
		if i == 0 {
			m.DrawBrailleLineWithStyle(cur, cur, st)
			continue
		}
		prev := canvas.Float64Point{X: float64(i - 1), Y: float64(pts[i-1].Value)}
		m.DrawBrailleLineWithStyle(prev, cur, st)
	}
}
