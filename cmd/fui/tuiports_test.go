package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/0xballo/fui/internal/app/reporting"
)

func TestReportExportResult(t *testing.T) {
	eur := func(v int64) reporting.Amount { return reporting.Amount{Minor: v, Currency: "EUR"} }
	in, out := eur(30000), eur(19000)
	grouped := reporting.GroupedResult{
		Period: "2026-06-01..2026-06-30", GroupBy: "month", Total: eur(11000),
		Rows: []reporting.GroupedRow{{Key: "2026-06-01", Total: eur(11000), Inflow: &in, Outflow: &out}},
	}
	u := reportUseCases{}

	var csv bytes.Buffer
	if err := u.ExportResult(&csv, "csv", grouped); err != nil {
		t.Fatalf("csv: %v", err)
	}
	if !strings.Contains(csv.String(), "2026-06-01,300.00,190.00,110.00") || !strings.Contains(csv.String(), "TOTAL,,,110.00") {
		t.Fatalf("csv = %q", csv.String())
	}

	var js bytes.Buffer
	if err := u.ExportResult(&js, "json", grouped); err != nil || !strings.Contains(js.String(), `"group_by":"month"`) {
		t.Fatalf("json = %q, %v", js.String(), err)
	}

	payees := reporting.PayeesResult{Total: eur(100), Rows: []reporting.PayeeRow{{Payee: "A", Total: eur(100), Count: 1}}}
	for _, data := range []any{
		reporting.SeriesResult{},
		reporting.NetWorthResult{},
		reporting.DebtResult{},
		reporting.InterestResult{},
		payees,
		reporting.BudgetResult{},
		reporting.ForecastResult{},
	} {
		for _, f := range []string{"csv", "json"} {
			if err := u.ExportResult(&bytes.Buffer{}, f, data); err != nil {
				t.Errorf("%T as %s: %v", data, f, err)
			}
		}
	}
	if err := u.ExportResult(&bytes.Buffer{}, "fui", grouped); err == nil {
		t.Error("unknown format accepted")
	}
	if err := u.ExportResult(&bytes.Buffer{}, "csv", 42); err == nil {
		t.Error("unsupported result accepted")
	}
}
