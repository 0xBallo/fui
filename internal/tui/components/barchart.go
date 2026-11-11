package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/0xballo/fui/internal/tui/theme"
)

// BarItem represents one line entry in a text bar chart.
type BarItem struct {
	Label        string
	Value        int64
	DisplayValue string
}

// BarChart renders horizontal textual bar graphs scaled to available terminal width.
type BarChart struct {
	items []BarItem
	width int
	theme theme.Theme
}

// NewBarChart constructs a chart initialized with an active visual theme.
func NewBarChart(th theme.Theme) BarChart {
	return BarChart{theme: th, width: 60}
}

// SetItems updates the data points rendered by the chart.
func (b *BarChart) SetItems(items []BarItem) {
	b.items = items
}

// SetWidth assigns the maximum horizontal character space for the chart.
func (b *BarChart) SetWidth(w int) {
	if w > 20 {
		b.width = w
	}
}

// View computes bar lengths proportional to the highest item and produces the text visualization.
func (b *BarChart) View() string {
	if len(b.items) == 0 {
		return b.theme.Muted.Render("(no data)")
	}

	maxVal, maxLabelLen, maxDisplayLen := b.calculateDimensions()
	if maxVal <= 0 {
		maxVal = 1
	}

	barSpace := b.width - maxLabelLen - maxDisplayLen - 4
	if barSpace < 5 {
		barSpace = 5
	}

	var lines []string
	for _, item := range b.items {
		lines = append(lines, b.renderRow(item, maxVal, maxLabelLen, maxDisplayLen, barSpace))
	}
	return strings.Join(lines, "\n")
}

func (b *BarChart) calculateDimensions() (maxVal int64, maxLabelLen int, maxDisplayLen int) {
	for _, item := range b.items {
		if item.Value > maxVal {
			maxVal = item.Value
		}
		if l := ansi.StringWidth(item.Label); l > maxLabelLen {
			maxLabelLen = l
		}
		if l := ansi.StringWidth(item.DisplayValue); l > maxDisplayLen {
			maxDisplayLen = l
		}
	}
	if maxLabelLen > 20 {
		maxLabelLen = 20
	}
	return maxVal, maxLabelLen, maxDisplayLen
}

func (b *BarChart) renderRow(item BarItem, maxVal int64, labelLen, displayLen, barSpace int) string {
	lbl := ansi.Truncate(item.Label, labelLen, "…")
	styledLabel := lipgloss.NewStyle().Width(labelLen).Render(lbl)

	valRatio := float64(max(0, item.Value)) / float64(maxVal)
	fillLen := int(valRatio * float64(barSpace))
	if item.Value > 0 && fillLen == 0 {
		fillLen = 1
	}

	barStr := strings.Repeat("█", fillLen)
	styledBar := b.theme.BarFill.Render(barStr)
	paddedBar := lipgloss.NewStyle().Width(barSpace).Render(styledBar)

	styledVal := lipgloss.NewStyle().Width(displayLen).Align(lipgloss.Right).Render(item.DisplayValue)

	return styledLabel + " " + paddedBar + " " + styledVal
}
