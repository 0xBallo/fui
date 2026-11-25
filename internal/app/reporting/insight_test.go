package reporting_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/recurring"
	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/clock"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

func newInsights(t *testing.T) (*fx, *reporting.Insights) {
	t.Helper()
	f := newFx(t)
	f.cfg.Locale = domain.LocaleENUS
	clk := clock.NewFixed(date("2026-03-15").In(time.UTC))
	rec := recurring.New(f.store, clk, noBackup{}, recurring.Config{Location: time.UTC})
	return f, reporting.NewInsights(reporting.NewAnalytics(f.store, clk, nil, f.cfg), rec, domain.InsightConfig{})
}

func seedInsights(t *testing.T, f *fx) {
	t.Helper()
	ctx := context.Background()
	var nf, gr domain.Payee
	var cat domain.Category
	err := f.store.WithTx(ctx, func(r app.Repos) (err error) {
		if nf, err = r.Payees().Insert(ctx, domain.Payee{Name: "Netflix"}); err != nil {
			return err
		}
		if gr, err = r.Payees().Insert(ctx, domain.Payee{Name: "Grocer"}); err != nil {
			return err
		}
		cat, err = r.Categories().Insert(ctx, domain.Category{Name: "Media", Kind: domain.CategoryExpense})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"2025-12-20", "2026-01-20", "2026-02-20"} {
		f.tx(t, f.main, d, -1_299, func(x *domain.Transaction) { x.PayeeID = nf.ID; x.CategoryID = cat.ID })
	}
	for _, d := range []string{"2025-12-03", "2026-01-04", "2026-02-02"} {
		f.tx(t, f.main, d, -3_000, func(x *domain.Transaction) { x.PayeeID = gr.ID })
	}
	f.tx(t, f.main, "2026-03-10", -9_000, func(x *domain.Transaction) { x.PayeeID = gr.ID })
}

func keyOf(t *testing.T, got []reporting.InsightView, kind string) string {
	t.Helper()
	i := slices.IndexFunc(got, func(v reporting.InsightView) bool { return v.Kind == kind })
	if i < 0 {
		t.Fatalf("no %s insight in %+v", kind, got)
	}
	return got[i].Key
}

func TestInsightsRunDismiss(t *testing.T) {
	ctx := context.Background()
	f, ins := newInsights(t)
	seedInsights(t, f)
	got, err := ins.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	key := keyOf(t, got, "anomaly")
	keyOf(t, got, "subscription")
	if got[slices.IndexFunc(got, func(v reporting.InsightView) bool { return v.Key == key })].Label != "Grocer" {
		t.Fatalf("label not resolved: %+v", got)
	}
	for range 2 { // dismissing twice is a no-op
		if err := ins.Dismiss(ctx, key); err != nil {
			t.Fatal(err)
		}
	}
	got, _ = ins.Run(ctx)
	if slices.ContainsFunc(got, func(v reporting.InsightView) bool { return v.Key == key }) {
		t.Fatalf("dismissed insight reappeared: %+v", got)
	}
	if err := ins.Dismiss(ctx, "nope"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown key err = %v", err)
	}
}

func TestInsightsAcceptSubscription(t *testing.T) {
	ctx := context.Background()
	f, ins := newInsights(t)
	seedInsights(t, f)
	got, _ := ins.Run(ctx)
	if _, err := ins.AcceptSubscription(ctx, keyOf(t, got, "anomaly")); err == nil {
		t.Fatal("accepting an anomaly must fail")
	}
	key := keyOf(t, got, "subscription")
	var recs []app.Recurrence
	for i, wantCreated := range []bool{true, false} {
		res, err := ins.AcceptSubscription(ctx, key)
		if err != nil {
			t.Fatal(err)
		}
		if res.Created != wantCreated {
			t.Fatalf("accept #%d created = %v", i, res.Created)
		}
		_ = f.store.Read(ctx, func(r app.Repos) (err error) { recs, err = r.Recurrences().List(ctx); return err })
		if len(recs) != 1 {
			t.Fatalf("recurrences = %d, want 1", len(recs))
		}
	}
	r := recs[0]
	if r.Amount.Minor() != -1_299 || r.Start != date("2026-03-20") || r.Rule.Unit != domain.RecurrenceMonths ||
		r.Rule.Every != 1 || r.Description != "Netflix" || r.AccountID != f.main.ID {
		t.Fatalf("recurrence = %+v", r)
	}
	got, _ = ins.Run(ctx)
	if slices.ContainsFunc(got, func(v reporting.InsightView) bool { return v.Kind == "subscription" }) {
		t.Fatalf("accepted subscription reappeared: %+v", got)
	}
}

func TestInsightsGate(t *testing.T) {
	f := newFx(t)
	f.cfg.Modules = must(module.Validate([]module.ID{module.Core}))
	ins := reporting.NewInsights(reporting.NewAnalytics(f.store, clock.NewFixed(date("2026-03-15").In(time.UTC)), nil, f.cfg), nil, domain.InsightConfig{})
	if _, err := ins.Run(context.Background()); err == nil {
		t.Fatal("expected module_disabled")
	}
}

func TestComparePeriods(t *testing.T) {
	ctx := context.Background()
	f := newFx(t)
	seedHistory(t, f)
	got, err := f.svc.ComparePeriods(ctx, reporting.CompareRequest{A: "last-month", B: "this-month", ByCategory: true})
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "compare", got)
	if _, err := f.svc.ComparePeriods(ctx, reporting.CompareRequest{A: "bogus"}); err == nil {
		t.Fatal("bad period must fail")
	}
	empty, err := f.svc.ComparePeriods(ctx, reporting.CompareRequest{A: "last-year", B: "last-year"})
	if err != nil || empty.Total.A.Currency != "EUR" {
		t.Fatalf("empty = %+v, %v", empty, err)
	}
}

func TestInsightsConvertForeignExpenses(t *testing.T) {
	ctx := context.Background()
	f, ins := newInsights(t)
	usd := must(domain.ParseCurrency("USD"))
	var acct domain.Account
	var shop domain.Payee
	err := f.store.WithTx(ctx, func(r app.Repos) (err error) {
		acct, err = r.Accounts().Insert(ctx, domain.Account{Name: "Dollars", Kind: domain.AccountChecking, Currency: usd, OpenedOn: date("2025-12-01")})
		if err != nil {
			return err
		}
		if shop, err = r.Payees().Insert(ctx, domain.Payee{Name: "Shop"}); err != nil {
			return err
		}
		return r.FXRates().Upsert(ctx, must(domain.NewFXRate(usd, date("2025-12-01"), must(domain.NewRate(domain.RateScale/2)))))
	})
	if err != nil {
		t.Fatal(err)
	}
	add := func(d string, minor int64) {
		f.tx(t, acct, d, minor, func(x *domain.Transaction) {
			x.PayeeID = shop.ID
			x.Amount = must(domain.NewMoney(minor, usd))
		})
	}
	for _, d := range []string{"2025-12-05", "2026-01-05", "2026-02-05"} {
		add(d, -2_000)
	}
	add("2026-03-10", -6_000)
	got, err := ins.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(got, func(v reporting.InsightView) bool { return v.Kind == "anomaly" })
	if i < 0 {
		t.Fatalf("no anomaly in %+v", got)
	}
	if a := got[i].Amount; a.Currency != "EUR" || a.Minor != 3_000 {
		t.Fatalf("anomaly amount = %+v, want 3000 EUR", a)
	}
}

func TestInsightsMissingRateExcludedAndWarned(t *testing.T) {
	ctx := context.Background()
	f, ins := newInsights(t)
	seedInsights(t, f)
	jpy := must(domain.ParseCurrency("JPY"))
	var acct domain.Account
	err := f.store.WithTx(ctx, func(r app.Repos) (err error) {
		acct, err = r.Accounts().Insert(ctx, domain.Account{Name: "Yen", Kind: domain.AccountCash, Currency: jpy, OpenedOn: date("2025-12-01")})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"2026-01-05", "2026-02-05"} {
		f.tx(t, acct, d, -500, func(x *domain.Transaction) { x.Amount = must(domain.NewMoney(-500, jpy)) })
	}
	rep, err := ins.Report(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Warnings) != 1 || !strings.Contains(rep.Warnings[0], "2 transactions") || !strings.Contains(rep.Warnings[0], "JPY") {
		t.Fatalf("warnings = %v", rep.Warnings)
	}
	keyOf(t, rep.Insights, "anomaly")
}

func TestInsightsSkipKnownRecurrence(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name string
		desc string
		seed func(f *fx, t *testing.T)
	}{
		{"payee name", "NETFLIX ", func(f *fx, t *testing.T) { seedInsights(t, f) }},
		{"description", "gym", func(f *fx, t *testing.T) {
			for _, d := range []string{"2025-12-20", "2026-01-20", "2026-02-20"} {
				f.tx(t, f.main, d, -1_299, func(x *domain.Transaction) { x.Description = "Gym" })
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, ins := newInsights(t)
			tc.seed(f, t)
			before, err := ins.Run(ctx)
			if err != nil {
				t.Fatal(err)
			}
			keyOf(t, before, "subscription")
			rule := must(domain.NewRecurrenceRule(1, domain.RecurrenceMonths, 20))
			err = f.store.WithTx(ctx, func(r app.Repos) error {
				_, err := r.Recurrences().Insert(ctx, app.Recurrence{
					AccountID: f.main.ID, Description: tc.desc, Amount: money(-1_299), Rule: rule,
					Start: date("2026-01-20"), NextDate: date("2026-03-20"),
				})
				return err
			})
			if err != nil {
				t.Fatal(err)
			}
			after, err := ins.Run(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if i := slices.IndexFunc(after, func(v reporting.InsightView) bool { return v.Kind == "subscription" && v.Amount.Minor == 1_299 }); i >= 0 {
				t.Fatalf("covered subscription still reported: %+v", after[i])
			}
		})
	}
}
