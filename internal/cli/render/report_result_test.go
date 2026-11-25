package render

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/domain"
)

var update = flag.Bool("update", false, "rewrite the golden files")

func amt(minor int64) reporting.Amount { return reporting.Amount{Minor: minor, Currency: "EUR"} }

func amtp(minor int64) *reporting.Amount { a := amt(minor); return &a }

func TestWriteReportResultGolden(t *testing.T) {
	permille := int64(-250)
	series := reporting.SeriesResult{Period: "2026-09-01..2026-09-30", Points: []reporting.Point{
		{Date: "2026-09-15", Amount: amt(1000)}, {Date: "2026-09-30", Amount: amt(-2050)},
	}}
	grouped := reporting.GroupedResult{
		Period: "2026-09-01..2026-09-30", GroupBy: "payee", Total: amt(3000),
		Rows: []reporting.GroupedRow{{Key: "Coop", Total: amt(2000)}, {Key: "(no payee)", Total: amt(1000)}},
	}
	cash := reporting.GroupedResult{
		Period: "2026-09-01..2026-09-30", GroupBy: "month", Total: amt(500),
		Rows: []reporting.GroupedRow{{Key: "2026-09-01", Total: amt(500), Inflow: amtp(1500), Outflow: amtp(1000)}},
	}
	cases := []struct {
		name string
		data any
	}{
		{"series", series},
		{"debt", reporting.DebtResult{Loan: "Car", SeriesResult: series}},
		{"interest", reporting.InterestResult{Period: "2026", Rows: []reporting.InterestRow{}}},
		{"payees", reporting.PayeesResult{Period: "2026-09", Total: amt(10), Rows: []reporting.PayeeRow{}}},
		{"budget", reporting.BudgetResult{Period: "2026-09", Mode: "cap", Lines: []reporting.BudgetLine{}, Uncovered: amt(0)}},
		{"forecast", reporting.ForecastResult{Start: "2026-09-01", Days: 2, Total: series.Points}},
		{"networth", reporting.NetWorthResult{Period: "2026", Points: []reporting.NetWorthPoint{
			{Date: "2026-09-30", Liquidity: amt(1000), Total: amt(1000)},
			{Date: "2026-10-31", Liquidity: amt(1000), Positions: amtp(500), Debts: amtp(-300), Total: amt(1200)},
		}}},
		{"compare", reporting.CompareReport{A: "a", B: "b", Rows: []reporting.CompareRowView{
			{Category: "Food", A: amt(400), B: amt(300), Delta: amt(-100), DeltaPermille: &permille},
		}, Total: reporting.CompareRowView{A: amt(400), B: amt(300), Delta: amt(-100)}}},
		{"grouped", grouped},
		{"cashflow", cash},
		{"saved", reporting.SavedResult{Name: "x", Kind: "spending", Data: grouped}},
	}
	for _, tc := range cases {
		for ext, f := range map[string]Format{"csv": CSV, "json": JSON} {
			t.Run(tc.name+"/"+ext, func(t *testing.T) {
				var buf bytes.Buffer
				if err := WriteReportResult(&buf, f, domain.LocaleENUS, tc.data); err != nil {
					t.Fatal(err)
				}
				path := filepath.Join("testdata", "report_result_"+tc.name+"."+ext)
				if *update {
					if err := os.MkdirAll("testdata", 0o750); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				want, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if got := buf.String(); got != string(want) {
					t.Errorf("output differs from %s:\n%s", path, got)
				}
			})
		}
	}
}

func TestWriteReportResultErrors(t *testing.T) {
	for name, tc := range map[string]struct {
		f    Format
		data any
	}{"table": {Table, reporting.SeriesResult{}}, "unknown type": {CSV, 42}} {
		t.Run(name, func(t *testing.T) {
			err := WriteReportResult(&bytes.Buffer{}, tc.f, domain.LocaleENUS, tc.data)
			var de *domain.Error
			if !errors.As(err, &de) || de.Code != domain.CodeInvalidValue {
				t.Fatalf("err = %v, want invalid_value", err)
			}
		})
	}
}

func TestGroupedAndNetWorthTables(t *testing.T) {
	grouped := reporting.GroupedResult{
		GroupBy: "category", Total: amt(3000),
		Rows: []reporting.GroupedRow{{Key: "Food", Total: amt(2000)}, {Key: "Fun", Total: amt(1000)}},
	}
	var buf bytes.Buffer
	if err := WriteGroupedReport(&buf, Table, domain.LocaleENUS, false, grouped, true); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"CATEGORY", "CHART", "TOTAL", "█"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("grouped table missing %q:\n%s", want, buf.String())
		}
	}
	buf.Reset()
	nw := reporting.NetWorthResult{Points: []reporting.NetWorthPoint{{Date: "2026-09-30", Liquidity: amt(100), Total: amt(100)}}}
	if err := WriteNetWorthReport(&buf, Table, domain.LocaleENUS, false, nw, true); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"LIQUIDITY", "POSITIONS", "ASSETS", "DEBTS", "CHART"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("networth table missing %q:\n%s", want, buf.String())
		}
	}
	buf.Reset()
	cmp := reporting.CompareReport{Total: reporting.CompareRowView{A: amt(1), B: amt(2), Delta: amt(1)}}
	if err := WriteCompareReport(&buf, Table, domain.LocaleENUS, false, cmp); err != nil || !strings.Contains(buf.String(), "TOTAL") {
		t.Fatalf("compare table = %q, %v", buf.String(), err)
	}
}

func TestPermilleText(t *testing.T) {
	neg, pos := int64(-125), int64(1005)
	for want, in := range map[string]*int64{"-": nil, "-12.5%": &neg, "100.5%": &pos} {
		if got := permilleText(in); got != want {
			t.Errorf("permilleText = %q, want %q", got, want)
		}
	}
}
