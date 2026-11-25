package reporting_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"pgregory.net/rapid"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/clock"
	"github.com/0xballo/fui/internal/domain"
)

type groupSeed struct {
	food, rent, fun domain.Category
	grocer, cafe    domain.Payee
	trip, work      domain.Tag
	march           domain.Period
}

// seedGroups inserts a small March 2026 history exercising every filter field and grouping.
func seedGroups(t testing.TB, f *fx) groupSeed {
	t.Helper()
	ctx := context.Background()
	var s groupSeed
	s.march = must(domain.NewPeriod(date("2026-03-01"), date("2026-03-31")))
	err := f.store.WithTx(ctx, func(r app.Repos) (err error) {
		for name, dst := range map[string]*domain.Category{"Food": &s.food, "Rent": &s.rent, "Fun": &s.fun} {
			if *dst, err = r.Categories().Insert(ctx, domain.Category{Name: name, Kind: domain.CategoryExpense}); err != nil {
				return err
			}
		}
		if s.grocer, err = r.Payees().Insert(ctx, domain.Payee{Name: "Grocer"}); err != nil {
			return err
		}
		if s.cafe, err = r.Payees().Insert(ctx, domain.Payee{Name: "Cafe"}); err != nil {
			return err
		}
		if s.trip, err = r.Tags().Insert(ctx, domain.Tag{Name: "trip"}); err != nil {
			return err
		}
		s.work, err = r.Tags().Insert(ctx, domain.Tag{Name: "work"})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	foodTrip := f.tx(t, f.main, "2026-03-02", -3_000, func(x *domain.Transaction) {
		x.PayeeID, x.CategoryID, x.Description = s.grocer.ID, s.food.ID, "weekly shop"
	})
	funTripWork := f.tx(t, f.main, "2026-03-09", -2_000, func(x *domain.Transaction) {
		x.PayeeID, x.CategoryID = s.cafe.ID, s.fun.ID
	})
	f.tx(t, f.sav, "2026-03-20", -50_000, func(x *domain.Transaction) { x.CategoryID = s.rent.ID })
	f.tx(t, f.main, "2026-03-21", 500, func(x *domain.Transaction) {
		x.PayeeID, x.CategoryID = s.cafe.ID, s.fun.ID
	})
	split := f.tx(t, f.main, "2026-03-25", -1_000, func(x *domain.Transaction) { x.PayeeID = s.grocer.ID })
	f.tx(t, f.main, "2026-03-26", -700, func(x *domain.Transaction) { x.Status = domain.StatusPending })
	f.tx(t, f.main, "2026-03-27", -9_999, func(x *domain.Transaction) { x.Kind = domain.KindTransfer })
	f.tx(t, f.main, "2026-03-28", 100_000, func(x *domain.Transaction) { x.Description = "salary" })
	err = f.store.WithTx(ctx, func(r app.Repos) error {
		if err := r.Tags().SetTags(ctx, foodTrip.ID, []domain.TagID{s.trip.ID}); err != nil {
			return err
		}
		if err := r.Tags().SetTags(ctx, funTripWork.ID, []domain.TagID{s.trip.ID, s.work.ID}); err != nil {
			return err
		}
		return r.Splits().ReplaceSplits(ctx, split.ID, []domain.Split{
			{Category: s.food.ID, Amount: money(-600)}, {Category: s.fun.ID, Amount: money(-400)},
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func rowsOf(res reporting.GroupedResult) map[string]int64 {
	out := map[string]int64{}
	for _, r := range res.Rows {
		out[r.Key] = r.Total.Minor
	}
	return out
}

func TestSpendingFilters(t *testing.T) {
	f := newFx(t)
	s := seedGroups(t, f)
	tests := []struct {
		name   string
		filter app.ListTxRequest
		want   int64
	}{
		{"none", app.ListTxRequest{}, 3_000 + 2_000 + 50_000 + 1_000 + 700},
		{"account", app.ListTxRequest{Accounts: []string{"Savings"}}, 50_000},
		{"category", app.ListTxRequest{Categories: []string{"Food"}}, 3_000 + 600},
		{"category by split row only", app.ListTxRequest{Categories: []string{"Fun"}}, 2_000 + 400},
		{"uncategorized", app.ListTxRequest{Uncategorized: true}, 700},
		{"tag", app.ListTxRequest{Tags: []string{"trip"}}, 3_000 + 2_000},
		{"tags or", app.ListTxRequest{Tags: []string{"work", "trip"}}, 3_000 + 2_000},
		{"payee", app.ListTxRequest{Payees: []string{"Grocer"}}, 3_000 + 1_000},
		{"min amount", app.ListTxRequest{MinAmount: "25"}, 3_000 + 50_000},
		{"max amount", app.ListTxRequest{MaxAmount: "10", Accounts: []string{"Main"}}, 700 + 1_000},
		{"status", app.ListTxRequest{Statuses: []string{"pending"}}, 700},
		{"kind transfer stays excluded", app.ListTxRequest{Kinds: []string{"transfer"}}, 0},
		{"text", app.ListTxRequest{Text: "weekly"}, 3_000},
		{"period fields ignored", app.ListTxRequest{From: "2020-01-01", To: "2020-01-02"}, 3_000 + 2_000 + 50_000 + 1_000 + 700},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, err := f.svc.Spending(context.Background(), reporting.ReportRequest{Period: s.march, Filter: tc.filter})
			if err != nil {
				t.Fatal(err)
			}
			if res.Total.Minor != tc.want {
				t.Errorf("total = %d, want %d (rows %v)", res.Total.Minor, tc.want, rowsOf(res))
			}
			var sum int64
			for _, r := range res.Rows {
				sum += r.Total.Minor
			}
			if sum != res.Total.Minor {
				t.Errorf("rows sum %d != total %d", sum, res.Total.Minor)
			}
		})
	}
}

func TestSpendingGroups(t *testing.T) {
	f := newFx(t)
	s := seedGroups(t, f)
	tests := []struct {
		by   domain.ReportGroup
		want map[string]int64
	}{
		{"", map[string]int64{"Rent": 50_000, "Food": 3_600, "Fun": 2_400, "Uncategorized": 700}},
		{domain.GroupCategory, map[string]int64{"Rent": 50_000, "Food": 3_600, "Fun": 2_400, "Uncategorized": 700}},
		{domain.GroupPayee, map[string]int64{"(no payee)": 50_700, "Grocer": 4_000, "Cafe": 2_000}},
		{domain.GroupAccount, map[string]int64{"Main": 6_700, "Savings": 50_000}},
		{domain.GroupTag, map[string]int64{"trip": 5_000, "work": 2_000, "untagged": 51_700}},
		{domain.GroupMonth, map[string]int64{"2026-03-01": 56_700}},
		{domain.GroupYear, map[string]int64{"2026-03-01": 56_700}},
	}
	for _, tc := range tests {
		t.Run(string(tc.by), func(t *testing.T) {
			res, err := f.svc.Spending(context.Background(), reporting.ReportRequest{Period: s.march, GroupBy: tc.by})
			if err != nil {
				t.Fatal(err)
			}
			if res.Total.Minor != 56_700 || res.Total.Currency != "EUR" {
				t.Errorf("total = %v, want 56700 EUR", res.Total)
			}
			if diff := cmp.Diff(tc.want, rowsOf(res)); diff != "" {
				t.Errorf("rows (-want +got):\n%s", diff)
			}
			for _, r := range res.Rows {
				if r.Inflow != nil || r.Outflow != nil {
					t.Errorf("spending row %q has inflow/outflow", r.Key)
				}
			}
		})
	}
}

func TestSpendingWeekBuckets(t *testing.T) {
	f := newFx(t)
	s := seedGroups(t, f)
	res, err := f.svc.Spending(context.Background(), reporting.ReportRequest{Period: s.march, GroupBy: domain.GroupWeek})
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, r := range res.Rows {
		keys = append(keys, r.Key)
	}
	// ISO weeks end on Sunday: the first bucket is clipped at the period start (Sunday 1st).
	want := []string{"2026-03-01", "2026-03-02", "2026-03-09", "2026-03-16", "2026-03-23", "2026-03-30"}
	if diff := cmp.Diff(want, keys); diff != "" {
		t.Errorf("week keys (-want +got):\n%s", diff)
	}
	if got := rowsOf(res); got["2026-03-02"] != 3_000 || got["2026-03-01"] != 0 {
		t.Errorf("week rows = %v", got)
	}
}

func TestCashFlowGroups(t *testing.T) {
	f := newFx(t)
	s := seedGroups(t, f)
	ctx := context.Background()
	res, err := f.svc.CashFlow(ctx, reporting.ReportRequest{Period: s.march})
	if err != nil {
		t.Fatal(err)
	}
	if res.GroupBy != "month" || len(res.Rows) != 1 {
		t.Fatalf("default grouping = %q with %d rows, want month with 1", res.GroupBy, len(res.Rows))
	}
	row := res.Rows[0]
	// inflow: salary 1000.00 + refund 5.00; outflow: 30+20+500+10+7 (transfer left out)
	if row.Inflow.Minor != 100_500 || row.Outflow.Minor != 56_700 || row.Total.Minor != 43_800 || res.Total.Minor != 43_800 {
		t.Errorf("month row = in %v out %v net %v total %v", row.Inflow, row.Outflow, row.Total, res.Total)
	}
	byCat, err := f.svc.CashFlow(ctx, reporting.ReportRequest{Period: s.march, GroupBy: domain.GroupCategory})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int64{"Rent": -50_000, "Uncategorized": 100_000 - 700, "Food": -3_600, "Fun": -2_400 + 500}
	if diff := cmp.Diff(want, rowsOf(byCat)); diff != "" {
		t.Errorf("by category (-want +got):\n%s", diff)
	}
	if byCat.Total.Minor != 43_800 {
		t.Errorf("total = %d", byCat.Total.Minor)
	}
	tagged, err := f.svc.CashFlow(ctx, reporting.ReportRequest{Period: s.march, GroupBy: domain.GroupTag})
	if err != nil {
		t.Fatal(err)
	}
	if got := rowsOf(tagged); got["trip"] != -5_000 || got["work"] != -2_000 {
		t.Errorf("by tag = %v", got)
	}
}

func TestGroupByInvalid(t *testing.T) {
	f := newFx(t)
	for _, call := range []func() error{
		func() error {
			_, err := f.svc.Spending(context.Background(), reporting.ReportRequest{GroupBy: "colour"})
			return err
		},
		func() error {
			_, err := f.svc.CashFlow(context.Background(), reporting.ReportRequest{GroupBy: "colour"})
			return err
		},
	} {
		var de *domain.Error
		if err := call(); !errors.As(err, &de) || de.Code != domain.CodeInvalidValue || de.Details["field"] != "group-by" {
			t.Errorf("err = %v, want invalid_value on group-by", err)
		}
	}
}

func TestFilterErrors(t *testing.T) {
	f := newFx(t)
	seedGroups(t, f)
	for name, flt := range map[string]app.ListTxRequest{
		"unknown category": {Categories: []string{"Nope"}},
		"unknown payee":    {Payees: []string{"Nope"}},
		"bad status":       {Statuses: []string{"nope"}},
	} {
		if _, err := f.svc.Spending(context.Background(), reporting.ReportRequest{Filter: flt}); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func TestTopPayeesFiltered(t *testing.T) {
	f := newFx(t)
	s := seedGroups(t, f)
	res, err := f.svc.TopPayees(context.Background(), reporting.ReportRequest{
		Period: s.march, Filter: app.ListTxRequest{Categories: []string{"Fun"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Cafe: 20.00 spent, 5.00 refunded; Grocer: the 4.00 split row only.
	got := map[string]int64{}
	counts := map[string]int{}
	for _, r := range res.Rows {
		got[r.Payee], counts[r.Payee] = r.Total.Minor, r.Count
	}
	if diff := cmp.Diff(map[string]int64{"Cafe": 1_500, "Grocer": 400}, got); diff != "" {
		t.Errorf("payees (-want +got):\n%s", diff)
	}
	if counts["Grocer"] != 1 || counts["Cafe"] != 2 {
		t.Errorf("counts = %v", counts)
	}
}

func TestBalanceGranularityAndFilter(t *testing.T) {
	f := newFx(t)
	s := seedGroups(t, f)
	ctx := context.Background()
	week, err := f.svc.BalanceOverTime(ctx, reporting.ReportRequest{Period: s.march, Account: "main", Granularity: domain.GranWeek})
	if err != nil {
		t.Fatal(err)
	}
	if len(week.Points) != 6 || week.Points[0].Date != "2026-03-01" || week.Points[5].Date != "2026-03-31" {
		t.Errorf("week points = %v", week.Points)
	}
	year, err := f.svc.BalanceOverTime(ctx, reporting.ReportRequest{Period: s.march, Account: "main", Granularity: domain.GranYear})
	if err != nil || len(year.Points) != 1 {
		t.Fatalf("year points = %v, %v", year.Points, err)
	}
	both, err := f.svc.BalanceOverTime(ctx, reporting.ReportRequest{
		Period: s.march, Filter: app.ListTxRequest{Accounts: []string{"Main", "Savings"}, Categories: []string{"Food"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	main, _ := f.svc.BalanceOverTime(ctx, reporting.ReportRequest{Period: s.march, Account: "Main"})
	sav, _ := f.svc.BalanceOverTime(ctx, reporting.ReportRequest{Period: s.march, Account: "Savings"})
	if got, want := both.Points[0].Amount.Minor, main.Points[0].Amount.Minor+sav.Points[0].Amount.Minor; got != want {
		t.Errorf("summed balance = %d, want %d (other filter fields are ignored)", got, want)
	}
	if _, err := f.svc.BalanceOverTime(ctx, reporting.ReportRequest{Period: s.march, Account: "main", Granularity: "decade"}); err == nil {
		t.Error("invalid granularity accepted")
	}
}

// TestSpendingMatchesMVP guards the default output against the MVP spending use case.
func TestSpendingMatchesMVP(t *testing.T) {
	f := newFx(t)
	seedHistory(t, f)
	march := must(domain.NewPeriod(date("2026-03-01"), date("2026-03-31")))
	ctx := context.Background()
	old, err := reporting.NewSpending(memSource{f.store}, clock.NewFixed(date("2026-03-15").In(time.UTC)))
	if err != nil {
		t.Fatal(err)
	}
	want, err := old.Run(ctx, reporting.SpendingParams{Period: march, MonthStartDay: 1, BaseCurrency: eur()})
	if err != nil {
		t.Fatal(err)
	}
	got, err := f.svc.Spending(ctx, reporting.ReportRequest{Period: march})
	if err != nil {
		t.Fatal(err)
	}
	wantRows := map[string]int64{}
	for _, r := range want.Rows {
		wantRows[r.CategoryName] = r.Total.Minor()
	}
	if diff := cmp.Diff(wantRows, rowsOf(got)); diff != "" || want.Total.Minor() != got.Total.Minor {
		t.Errorf("new vs MVP spending (-mvp +new):\n%s", diff)
	}
}

func TestGroupedSumsProperty(t *testing.T) {
	groups := []domain.ReportGroup{domain.GroupCategory, domain.GroupPayee, domain.GroupAccount, domain.GroupMonth, domain.GroupWeek, domain.GroupYear}
	rapid.Check(t, func(rt *rapid.T) {
		f := newFx(t)
		s := seedGroups(t, f)
		for range rapid.IntRange(0, 12).Draw(rt, "extra") {
			acct := rapid.SampledFrom([]domain.Account{f.main, f.sav}).Draw(rt, "acct")
			day := rapid.IntRange(1, 31).Draw(rt, "day")
			amount := rapid.Int64Range(-5_000, 5_000).Draw(rt, "amount")
			cat := rapid.SampledFrom([]domain.CategoryID{0, s.food.ID, s.rent.ID, s.fun.ID}).Draw(rt, "cat")
			py := rapid.SampledFrom([]domain.PayeeID{0, s.grocer.ID, s.cafe.ID}).Draw(rt, "payee")
			f.tx(t, acct, "2026-03-"+twoDigits(day), amount, func(x *domain.Transaction) { x.CategoryID, x.PayeeID = cat, py })
		}
		flt := app.ListTxRequest{}
		if rapid.Bool().Draw(rt, "filtered") {
			flt.Categories = []string{rapid.SampledFrom([]string{"Food", "Rent", "Fun"}).Draw(rt, "fcat")}
		}
		base := reporting.ReportRequest{Period: s.march, Filter: flt}
		want, err := f.svc.Spending(context.Background(), base)
		if err != nil {
			rt.Fatal(err)
		}
		wantCF, err := f.svc.CashFlow(context.Background(), base)
		if err != nil {
			rt.Fatal(err)
		}
		for _, by := range groups {
			req := base
			req.GroupBy = by
			for kind, run := range map[string]func(context.Context, reporting.ReportRequest) (reporting.GroupedResult, error){"spending": f.svc.Spending, "cashflow": f.svc.CashFlow} {
				got, err := run(context.Background(), req)
				if err != nil {
					rt.Fatal(err)
				}
				var sum int64
				for _, r := range got.Rows {
					sum += r.Total.Minor
				}
				total := want.Total.Minor
				if kind == "cashflow" {
					total = wantCF.Total.Minor
				}
				if sum != total || got.Total.Minor != total {
					rt.Fatalf("%s by %s: rows sum %d, total %d, ungrouped %d", kind, by, sum, got.Total.Minor, total)
				}
			}
		}
	})
}

func twoDigits(n int) string {
	return string([]byte{byte('0' + n/10), byte('0' + n%10)})
}

func TestGroupedGolden(t *testing.T) {
	f := newFx(t)
	s := seedGroups(t, f)
	ctx := context.Background()
	trip := app.ListTxRequest{Accounts: []string{"Main"}, Categories: []string{"Food", "Fun"}}
	sp, err := f.svc.Spending(ctx, reporting.ReportRequest{Period: s.march, Filter: trip, GroupBy: domain.GroupPayee})
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "spending_grouped", sp)
	cf, err := f.svc.CashFlow(ctx, reporting.ReportRequest{Period: s.march, Filter: trip, GroupBy: domain.GroupWeek})
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "cashflow_grouped", cf)
	pay, err := f.svc.TopPayees(ctx, reporting.ReportRequest{Period: s.march, Filter: trip})
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "payees_filtered", pay)
	bal, err := f.svc.BalanceOverTime(ctx, reporting.ReportRequest{Period: s.march, Filter: app.ListTxRequest{Accounts: []string{"Main"}}, Granularity: domain.GranWeek})
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "balance_weekly", bal)
}
