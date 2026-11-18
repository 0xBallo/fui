package report

import (
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/tui/components"
)

func pts(vals ...int64) []reporting.Point {
	out := make([]reporting.Point, len(vals))
	for i, v := range vals {
		out[i] = reporting.Point{Date: "2026-0" + string(rune('1'+i)) + "-01", Amount: reporting.Amount{Minor: v, Currency: "EUR"}}
	}
	return out
}

func cp(vals ...int64) []components.Point {
	out := make([]components.Point, len(vals))
	for i, v := range vals {
		out[i] = components.Point{Label: "2026-0" + string(rune('1'+i)) + "-01", Value: v}
	}
	return out
}

func TestLineSeries(t *testing.T) {
	tests := []struct {
		name string
		kind string
		data any
		want []components.Series
	}{
		{
			"net worth", reporting.KindNetWorth,
			reporting.SeriesResult{Points: pts(1, 2)},
			[]components.Series{{Name: "Net worth", Points: cp(1, 2)}},
		},
		{"net worth full", reporting.KindNetWorth, reporting.NetWorthResult{Points: []reporting.NetWorthPoint{
			{Date: "2026-01-01", Total: reporting.Amount{Minor: 5}, Liquidity: reporting.Amount{Minor: 3}},
		}}, []components.Series{
			{Name: "Total", Points: []components.Point{{Label: "2026-01-01", Value: 5}}},
			{Name: "Liquidity", Points: []components.Point{{Label: "2026-01-01", Value: 3}}},
		}},
		{
			"balance", reporting.KindBalance,
			reporting.SeriesResult{Points: pts(7)},
			[]components.Series{{Name: "Balance", Points: cp(7)}},
		},
		{
			"forecast", reporting.KindForecast,
			reporting.ForecastResult{Total: pts(4, 5)},
			[]components.Series{{Name: "Projected balance", Points: cp(4, 5)}},
		},
		{
			"debt", reporting.KindDebt,
			reporting.DebtResult{SeriesResult: reporting.SeriesResult{Points: pts(-9)}},
			[]components.Series{{Name: "Remaining debt", Points: cp(-9)}},
		},
		{"payees has none", reporting.KindPayees, reporting.PayeesResult{}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := lineSeries(tc.kind, tc.data)
			if diff := cmp.Diff(tc.want, got, cmpopts.IgnoreFields(components.Series{}, "Style")); diff != "" {
				t.Fatalf("series mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestChartViewHiddenWhenShort(t *testing.T) {
	m := newTestModel()
	m.result = reportResult{kind: reporting.KindBalance, data: reporting.SeriesResult{Points: pts(1, 2, 3)}}
	if got := m.chartView(80, minChartScreen-1); got != "" {
		t.Fatalf("chart shown at small height:\n%s", got)
	}
	if got := m.chartView(80, 24); got == "" {
		t.Fatal("chart hidden at height 24")
	}
	if h := chartHeight(40); h != maxChartHeight {
		t.Fatalf("chartHeight(40) = %d, want %d", h, maxChartHeight)
	}
}

func TestChartViewGolden(t *testing.T) {
	m := newTestModel()
	m.result = reportResult{kind: reporting.KindBalance, data: reporting.SeriesResult{Points: pts(100000, 150000, 120000, 180000)}}
	golden.RequireEqual(t, []byte(ansi.Strip(m.chartView(60, 24))))
}
