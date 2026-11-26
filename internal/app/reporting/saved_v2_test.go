package reporting_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/domain"
)

func TestPeriodSpecs(t *testing.T) {
	today := date("2026-03-15")
	for _, tc := range []struct {
		in       string
		day      int
		from, to string
	}{
		{"2026-02", 1, "2026-02-01", "2026-02-28"},
		{"2024-02", 1, "2024-02-01", "2024-02-29"},
		{"2026-03", 25, "2026-02-25", "2026-03-24"},
		{"2026-Q1", 1, "2026-01-01", "2026-03-31"},
		{"2026-Q2", 1, "2026-04-01", "2026-06-30"},
		{"2026-q3", 1, "2026-07-01", "2026-09-30"},
		{"2026-Q4", 1, "2026-10-01", "2026-12-31"},
		{"2024", 1, "2024-01-01", "2024-12-31"},
		{"last-month", 1, "2026-02-01", "2026-02-28"},
		{"this-year", 1, "2026-01-01", "2026-12-31"},
	} {
		p, err := reporting.ResolvePeriodSpec(tc.in, today, tc.day)
		if err != nil {
			t.Errorf("%s: %v", tc.in, err)
			continue
		}
		if p.From().String() != tc.from || p.To().String() != tc.to {
			t.Errorf("%s = %s..%s, want %s..%s", tc.in, p.From(), p.To(), tc.from, tc.to)
		}
		if !reporting.ValidPeriodSpec(tc.in) {
			t.Errorf("ValidPeriodSpec(%q) = false", tc.in)
		}
	}
	for _, bad := range []string{"", "2026-13", "2026-00", "2026-Q5", "2026-Q0", "26", "2026-1", "next-month", "0000", "2026-03-01"} {
		if reporting.ValidPeriodSpec(bad) {
			t.Errorf("ValidPeriodSpec(%q) = true", bad)
		}
		if _, err := reporting.ResolvePeriodSpec(bad, today, 1); err == nil {
			t.Errorf("ResolvePeriodSpec(%q) accepted", bad)
		}
	}
}

func TestSavePlanValidation(t *testing.T) {
	f := newFx(t)
	seedHistory(t, f)
	ctx := context.Background()
	for name, tc := range map[string]struct {
		kind string
		spec reporting.SavedSpec
	}{
		"unknown group-by":    {reporting.KindSpending, reporting.SavedSpec{GroupBy: "colour"}},
		"unknown granularity": {reporting.KindBalance, reporting.SavedSpec{Granularity: "decade"}},
		"forecast month":      {reporting.KindForecast, reporting.SavedSpec{Granularity: "month"}},
		"bad horizon":         {reporting.KindForecast, reporting.SavedSpec{Horizon: "soon"}},
		"absolute from":       {reporting.KindSpending, reporting.SavedSpec{Filter: &app.ListTxRequest{From: "2026-01-01"}}},
		"absolute month":      {reporting.KindSpending, reporting.SavedSpec{Filter: &app.ListTxRequest{Month: "2026-01"}}},
		"absolute year":       {reporting.KindSpending, reporting.SavedSpec{Filter: &app.ListTxRequest{Year: 2026}}},
		"unknown account":     {reporting.KindSpending, reporting.SavedSpec{Filter: &app.ListTxRequest{Accounts: []string{"nope"}}}},
		"bad compare period":  {reporting.KindCompare, reporting.SavedSpec{Compare: &reporting.CompareSpec{A: "2026-13", B: "this-month"}}},
		"bad version":         {reporting.KindSpending, reporting.SavedSpec{Version: 3}},
		"bad period":          {reporting.KindSpending, reporting.SavedSpec{Period: "2026-Q5"}},
	} {
		if _, err := f.svc.PlanSave(ctx, "r", tc.kind, tc.spec); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	p, err := f.svc.PlanSave(ctx, "ok", reporting.KindCompare, reporting.SavedSpec{Compare: &reporting.CompareSpec{A: "2026-Q1", B: "2025"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Report.ParamsJSON; got != `{"version":2,"compare":{"a":"2026-Q1","b":"2025"}}` {
		t.Errorf("params = %s", got)
	}
}

func saveSpec(t *testing.T, f *fx, name, kind string, spec reporting.SavedSpec) {
	t.Helper()
	ctx := context.Background()
	p, err := f.svc.PlanSave(ctx, name, kind, spec)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ApplySave(ctx, p); err != nil {
		t.Fatal(err)
	}
}

func TestSavedV1ParamsStillRun(t *testing.T) {
	f := newFx(t)
	seedHistory(t, f)
	ctx := context.Background()
	err := f.store.WithTx(ctx, func(r app.Repos) error {
		_, err := r.SavedReports().Insert(ctx, app.SavedReport{Name: "old", Kind: reporting.KindSpending, ParamsJSON: `{"period":"this-month","account":"Main"}`})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := f.svc.Run(ctx, "old")
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Data.(reporting.SpendingResult).Total.Minor(); got != 2_000 {
		t.Fatalf("v1 spending total = %d, want 2000", got)
	}
}

func TestSavedV2MatchesDirect(t *testing.T) {
	f := newFx(t)
	seedHistory(t, f)
	ctx := context.Background()
	filter := &app.ListTxRequest{Accounts: []string{"Main"}, Categories: []string{"Food"}}
	saveSpec(t, f, "sp", reporting.KindSpending, reporting.SavedSpec{Period: "last-3-months", Filter: filter, GroupBy: "month"})
	saveSpec(t, f, "cf", reporting.KindCashFlow, reporting.SavedSpec{Period: "this-year", GroupBy: "category", Account: "Main"})
	saveSpec(t, f, "cmp", reporting.KindCompare, reporting.SavedSpec{Compare: &reporting.CompareSpec{A: "2026-02", B: "this-month"}, Filter: filter})
	saveSpec(t, f, "cmpdef", reporting.KindCompare, reporting.SavedSpec{})
	saveSpec(t, f, "fc", reporting.KindForecast, reporting.SavedSpec{Horizon: "3w", Granularity: "week"})

	three := must(domain.NewPeriod(date("2025-12-01"), date("2026-02-28")))
	wantSp, err := f.svc.Spending(ctx, reporting.ReportRequest{Period: three, Filter: *filter, GroupBy: domain.GroupMonth})
	if err != nil {
		t.Fatal(err)
	}
	gotSp, err := f.svc.Run(ctx, "sp")
	if err != nil {
		t.Fatal(err)
	}
	if d := cmp.Diff(wantSp, gotSp.Data); d != "" {
		t.Errorf("spending differs:\n%s", d)
	}
	if len(wantSp.Rows) == 0 {
		t.Error("spending fixture is empty")
	}

	year := must(domain.NewPeriod(date("2026-01-01"), date("2026-12-31")))
	wantCf, err := f.svc.CashFlow(ctx, reporting.ReportRequest{Period: year, GroupBy: domain.GroupCategory, Filter: app.ListTxRequest{Accounts: []string{"Main"}}})
	if err != nil {
		t.Fatal(err)
	}
	gotCf, err := f.svc.Run(ctx, "cf")
	if err != nil {
		t.Fatal(err)
	}
	if d := cmp.Diff(wantCf, gotCf.Data); d != "" {
		t.Errorf("cash flow differs:\n%s", d)
	}

	wantCmp, err := f.svc.ComparePeriods(ctx, reporting.CompareRequest{A: "2026-02", B: "this-month", ByCategory: true, Filter: *filter})
	if err != nil {
		t.Fatal(err)
	}
	gotCmp, err := f.svc.Run(ctx, "cmp")
	if err != nil {
		t.Fatal(err)
	}
	if d := cmp.Diff(wantCmp, gotCmp.Data); d != "" {
		t.Errorf("compare differs:\n%s", d)
	}
	golden(t, "compare_saved", gotCmp)

	wantDef, err := f.svc.ComparePeriods(ctx, reporting.CompareRequest{ByCategory: true})
	if err != nil {
		t.Fatal(err)
	}
	gotDef, err := f.svc.Run(ctx, "cmpdef")
	if err != nil {
		t.Fatal(err)
	}
	if d := cmp.Diff(wantDef, gotDef.Data); d != "" {
		t.Errorf("default compare differs:\n%s", d)
	}

	if _, err := f.svc.Run(ctx, "fc"); err != nil {
		t.Errorf("forecast: %v", err)
	}
}

func TestCompareFilter(t *testing.T) {
	f := newFx(t)
	seedHistory(t, f)
	ctx := context.Background()
	f.tx(t, f.sav, "2026-03-08", -700, nil)
	all, err := f.svc.ComparePeriods(ctx, reporting.CompareRequest{A: "2026-02", B: "2026-03", ByCategory: true})
	if err != nil {
		t.Fatal(err)
	}
	onlyMain, err := f.svc.ComparePeriods(ctx, reporting.CompareRequest{A: "2026-02", B: "2026-03", ByCategory: true, Filter: app.ListTxRequest{Accounts: []string{"Main"}}})
	if err != nil {
		t.Fatal(err)
	}
	if all.Total.B.Minor-onlyMain.Total.B.Minor != 700 {
		t.Fatalf("filtered total = %d, unfiltered = %d", onlyMain.Total.B.Minor, all.Total.B.Minor)
	}
	golden(t, "compare_filtered", onlyMain)
}
