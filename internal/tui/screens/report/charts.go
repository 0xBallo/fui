package report

import (
	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/tui/components"
)

const (
	// maxChartHeight caps the line chart rows; minChartScreen is the screen height below which
	// the chart is hidden so the table keeps room.
	maxChartHeight = 12
	minChartScreen = 18
)

// lineSeries maps a loaded result to the series its line chart draws; it returns nil for kinds
// without a line chart. The net worth use case exposes only the total, so no liquidity series
// is drawn until the port returns the full reporting.NetWorthResult.
func lineSeries(kind string, data any) []components.Series {
	switch d := data.(type) {
	case reporting.SeriesResult:
		if kind == reporting.KindBalance || kind == reporting.KindNetWorth {
			return []components.Series{pointSeries(seriesName(kind), d.Points)}
		}
	case reporting.DebtResult:
		return []components.Series{pointSeries("Remaining debt", d.Points)}
	case reporting.ForecastResult:
		return []components.Series{pointSeries("Projected balance", d.Total)}
	case reporting.NetWorthResult:
		total := make([]components.Point, len(d.Points))
		liquidity := make([]components.Point, len(d.Points))
		for i, p := range d.Points {
			total[i] = components.Point{Label: p.Date, Value: p.Total.Minor}
			liquidity[i] = components.Point{Label: p.Date, Value: p.Liquidity.Minor}
		}
		return []components.Series{{Name: "Total", Points: total}, {Name: "Liquidity", Points: liquidity}}
	}
	return nil
}

func seriesName(kind string) string {
	if kind == reporting.KindNetWorth {
		return "Net worth"
	}
	return "Balance"
}

func pointSeries(name string, pts []reporting.Point) components.Series {
	out := make([]components.Point, len(pts))
	for i, p := range pts {
		out[i] = components.Point{Label: p.Date, Value: p.Amount.Minor}
	}
	return components.Series{Name: name, Points: out}
}

// chartHeight is the line chart height for a screen of height h, or 0 when it must be hidden.
func chartHeight(h int) int {
	if h < minChartScreen {
		return 0
	}
	return min(maxChartHeight, h/2)
}

// currencyOf returns the currency of the first point of data, used to format the Y axis.
func currencyOf(data any) string {
	switch d := data.(type) {
	case reporting.SeriesResult:
		if len(d.Points) > 0 {
			return d.Points[0].Amount.Currency
		}
	case reporting.DebtResult:
		if len(d.Points) > 0 {
			return d.Points[0].Amount.Currency
		}
	case reporting.ForecastResult:
		if len(d.Total) > 0 {
			return d.Total[0].Amount.Currency
		}
	case reporting.NetWorthResult:
		if len(d.Points) > 0 {
			return d.Points[0].Total.Currency
		}
	}
	return ""
}

// chartView renders the line chart for the loaded report, or "" when the kind has none, there is
// no data, or the screen is too short.
func (m *Model) chartView(width, height int) string {
	ch := chartHeight(height)
	series := lineSeries(m.result.kind, m.result.data)
	if len(series) == 0 {
		return m.renderBars(barItems(m.th, m.fmtAmount, m.result.data), width, height)
	}
	if ch == 0 || len(series) == 0 {
		return ""
	}
	cur := currencyOf(m.result.data)
	m.line.SetSize(width, ch)
	m.line.SetSeries(series...)
	m.line.SetYFormatter(func(v int64) string {
		return m.fmtAmount(reporting.Amount{Minor: v, Currency: cur})
	})
	return m.line.View()
}
