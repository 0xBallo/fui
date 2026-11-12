package reporting_test

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"pgregory.net/rapid"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/lending"
	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/clock"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
	"github.com/0xballo/fui/internal/testutil/memstore"
)

var update = flag.Bool("update", false, "rewrite golden files")

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func date(s string) domain.Date  { return must(domain.ParseDate(s)) }
func eur() domain.Currency       { return must(domain.ParseCurrency("EUR")) }
func money(m int64) domain.Money { return must(domain.NewMoney(m, eur())) }

type noBackup struct{}

func (noBackup) Backup(context.Context, string) (string, error) { return "", nil }
func (noBackup) Restore(context.Context, string) error          { return nil }

type fx struct {
	store *memstore.Store
	svc   *reporting.Analytics
	cfg   reporting.Config
	main  domain.Account
	sav   domain.Account
}

func newFx(t testing.TB, mods ...module.ID) *fx {
	t.Helper()
	ctx := context.Background()
	set := must(module.Validate(append([]module.ID{module.Core, module.Recurring, module.Analytics}, mods...)))
	f := &fx{store: memstore.New(), cfg: reporting.Config{BaseCurrency: eur(), Location: time.UTC, Modules: set}}
	f.svc = reporting.NewAnalytics(f.store, clock.NewFixed(date("2026-03-15").In(time.UTC)), memSource{f.store}, f.cfg)
	err := f.store.WithTx(ctx, func(r app.Repos) (err error) {
		f.main, err = r.Accounts().Insert(ctx, domain.Account{Name: "Main", Kind: domain.AccountChecking, Currency: eur(), OpeningBalance: money(100_000), OpenedOn: date("2025-12-01")})
		if err != nil {
			return err
		}
		f.sav, err = r.Accounts().Insert(ctx, domain.Account{Name: "Savings", Kind: domain.AccountSavings, Currency: eur(), OpeningBalance: money(50_000), OpenedOn: date("2025-12-01")})
		if err != nil {
			return err
		}
		_, err = r.Accounts().Insert(ctx, domain.Account{Name: "Old", Kind: domain.AccountChecking, Currency: eur(), OpeningBalance: money(10_000), OpenedOn: date("2025-12-01"), Archived: true})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *fx) tx(t testing.TB, acct domain.Account, d string, minor int64, mut func(*domain.Transaction)) domain.Transaction {
	t.Helper()
	tr := domain.Transaction{AccountID: acct.ID, Date: date(d), Amount: money(minor), Status: domain.StatusBooked, Kind: domain.KindPayment}
	if mut != nil {
		mut(&tr)
	}
	var out domain.Transaction
	err := f.store.WithTx(context.Background(), func(r app.Repos) (err error) {
		out, err = r.Transactions().Insert(context.Background(), tr)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func golden(t *testing.T, name string, v any) {
	t.Helper()
	got, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	path := filepath.Join("testdata", name+".golden.json")
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != string(got) {
		t.Errorf("%s differs from golden:\n%s", name, got)
	}
}

func seedHistory(t *testing.T, f *fx) {
	t.Helper()
	ctx := context.Background()
	var py domain.Payee
	var food, rent domain.Category
	err := f.store.WithTx(ctx, func(r app.Repos) (err error) {
		if py, err = r.Payees().Insert(ctx, domain.Payee{Name: "Grocer"}); err != nil {
			return err
		}
		if _, err = r.Payees().Insert(ctx, domain.Payee{Name: "Cafe"}); err != nil {
			return err
		}
		if food, err = r.Categories().Insert(ctx, domain.Category{Name: "Food", Kind: domain.CategoryExpense}); err != nil {
			return err
		}
		rent, err = r.Categories().Insert(ctx, domain.Category{Name: "Rent", Kind: domain.CategoryExpense})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = rent
	for _, m := range []string{"2025-12", "2026-01", "2026-02"} {
		f.tx(t, f.main, m+"-10", -3_000, func(x *domain.Transaction) { x.PayeeID = py.ID; x.CategoryID = food.ID })
	}
	f.tx(t, f.main, "2026-03-05", -1_500, func(x *domain.Transaction) { x.PayeeID = py.ID; x.CategoryID = food.ID })
	f.tx(t, f.main, "2026-03-06", -500, func(x *domain.Transaction) { x.PayeeID = py.ID + 1; x.CategoryID = food.ID })
	f.tx(t, f.main, "2026-03-07", 200_000, nil)
}

func TestReportsGolden(t *testing.T) {
	f := newFx(t)
	seedHistory(t, f)
	ctx := context.Background()
	year := must(domain.NewPeriod(date("2026-01-01"), date("2026-03-31")))
	nw, err := f.svc.NetWorth(ctx, reporting.ReportRequest{Period: year})
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "net_worth", nw)
	bal, err := f.svc.BalanceOverTime(ctx, reporting.ReportRequest{Period: year, Account: "main"})
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "balance", bal)
	pay, err := f.svc.TopPayees(ctx, reporting.ReportRequest{Period: year, Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "payees", pay)
}

func TestNetWorthIncludesArchived(t *testing.T) {
	f := newFx(t)
	nw, err := f.svc.NetWorth(context.Background(), reporting.ReportRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if got := nw.Points[0].Amount.Minor; got != 160_000 {
		t.Fatalf("net worth = %d, want 160000 (archived account included)", got)
	}
}

func TestNetWorthUnchangedByFeelessTransfer(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		f := newFx(t)
		amount := rapid.Int64Range(1, 90_000).Draw(rt, "amount")
		d := rapid.IntRange(1, 28).Draw(rt, "day")
		at := time.Date(2026, 3, d, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
		req := reporting.ReportRequest{Period: must(domain.NewPeriod(date("2026-03-01"), date("2026-03-31")))}
		before, err := f.svc.NetWorth(context.Background(), req)
		if err != nil {
			rt.Fatal(err)
		}
		f.tx(t, f.main, at, -amount, func(x *domain.Transaction) { x.Kind = domain.KindTransfer })
		f.tx(t, f.sav, at, amount, func(x *domain.Transaction) { x.Kind = domain.KindTransfer })
		after, err := f.svc.NetWorth(context.Background(), req)
		if err != nil {
			rt.Fatal(err)
		}
		if before.Points[0] != after.Points[0] {
			rt.Fatalf("net worth %v -> %v", before.Points[0], after.Points[0])
		}
	})
}

func TestForecastGolden(t *testing.T) {
	f := newFx(t, module.Loans)
	seedHistory(t, f)
	ctx := context.Background()
	rule := must(domain.NewRecurrenceRule(1, domain.RecurrenceMonths, 20))
	err := f.store.WithTx(ctx, func(r app.Repos) error {
		_, err := r.Recurrences().Insert(ctx, app.Recurrence{AccountID: f.main.ID, Description: "Rent", Amount: money(-60_000), Rule: rule, Start: date("2026-01-20"), NextDate: date("2026-03-20")})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	ls := lending.New(f.store, clock.NewFixed(date("2026-03-15").In(time.UTC)), noBackup{}, lending.Config{Location: time.UTC})
	plan, err := ls.PlanAdd(ctx, f.cfg.Modules, lending.AddRequest{
		Name: "Car", Kind: "loan", Principal: "1200", Disbursed: "2026-03-01", TermMonths: 12, RateType: "fixed", Rate: "2", Debit: "Main",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ls.ApplyAdd(ctx, plan); err != nil {
		t.Fatal(err)
	}
	res, err := f.svc.Forecast(ctx, reporting.ForecastRequest{HorizonDays: 30})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Total) != 31 {
		t.Fatalf("total points = %d, want 31", len(res.Total))
	}
	// Keep the golden readable: first, last and the days where something moves.
	var moves []reporting.Point
	for i, p := range res.Total {
		if i == 0 || i == len(res.Total)-1 || p.Amount != res.Total[i-1].Amount {
			moves = append(moves, p)
		}
	}
	res.Total = moves
	for i := range res.Accounts {
		res.Accounts[i].Series = res.Accounts[i].Series[len(res.Accounts[i].Series)-1:]
	}
	golden(t, "forecast", res)
}

func TestForecastBudgetReplacesAverage(t *testing.T) {
	f := newFx(t, module.Budget)
	seedHistory(t, f)
	ctx := context.Background()
	base, err := f.svc.Forecast(ctx, reporting.ForecastRequest{HorizonDays: 31})
	if err != nil {
		t.Fatal(err)
	}
	err = f.store.WithTx(ctx, func(r app.Repos) error {
		cats, err := r.Categories().List(ctx)
		if err != nil {
			return err
		}
		_, err = r.Budgets().Insert(ctx, domain.Budget{
			Name: "Food", Mode: domain.BudgetCap, Amount: money(31_000), Targets: []domain.BudgetTarget{{CategoryID: cats[0].ID}},
			Thresholds: domain.DefaultBudgetThresholds,
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	with, err := f.svc.Forecast(ctx, reporting.ForecastRequest{HorizonDays: 31})
	if err != nil {
		t.Fatal(err)
	}
	// The average is 3000/month, the cap 31000: over a month the cap spends about 28000 more.
	if d := base.Total[31].Amount.Minor - with.Total[31].Amount.Minor; d < 27_000 {
		t.Fatalf("budget did not replace the average: delta %d", d)
	}
}

func TestModuleGates(t *testing.T) {
	f := newFx(t)
	ctx := context.Background()
	for name, call := range map[string]func() error{
		"debt":   func() error { _, err := f.svc.RemainingDebt(ctx, reporting.ReportRequest{Loan: "x"}); return err },
		"budget": func() error { _, err := f.svc.BudgetVsActual(ctx, reporting.ReportRequest{}); return err },
	} {
		if err := call(); !errors.Is(err, app.ErrModuleDisabled) {
			t.Errorf("%s err = %v, want module_disabled", name, err)
		}
	}
	off := reporting.NewAnalytics(f.store, clock.NewFixed(date("2026-03-15").In(time.UTC)), nil, reporting.Config{BaseCurrency: eur()})
	if _, err := off.Forecast(ctx, reporting.ForecastRequest{}); !errors.Is(err, app.ErrModuleDisabled) {
		t.Errorf("forecast without recurring err = %v", err)
	}
	// Forecast belongs to recurring alone: the expense preset (no analytics) runs it.
	expense := reporting.Config{BaseCurrency: eur(), Location: time.UTC, Modules: must(module.Preset("expense"))}
	exp := reporting.NewAnalytics(f.store, clock.NewFixed(date("2026-03-15").In(time.UTC)), memSource{f.store}, expense)
	if _, err := exp.Forecast(ctx, reporting.ForecastRequest{HorizonDays: 7}); err != nil {
		t.Errorf("forecast under expense preset: %v", err)
	}
}

func TestLoanReportsAndBudgetGolden(t *testing.T) {
	f := newFx(t, module.Loans, module.Budget)
	seedHistory(t, f)
	ctx := context.Background()
	ls := lending.New(f.store, clock.NewFixed(date("2026-03-15").In(time.UTC)), noBackup{}, lending.Config{Location: time.UTC})
	plan, err := ls.PlanAdd(ctx, f.cfg.Modules, lending.AddRequest{
		Name: "Mortgage", Kind: "mortgage", Principal: "12000", Disbursed: "2026-01-01", TermMonths: 12, RateType: "fixed", Rate: "3,5", Debit: "Main",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ls.ApplyAdd(ctx, plan); err != nil {
		t.Fatal(err)
	}
	q1 := must(domain.NewPeriod(date("2026-01-01"), date("2026-03-31")))
	debt, err := f.svc.RemainingDebt(ctx, reporting.ReportRequest{Period: q1, Loan: "mort"})
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "debt", debt)
	interest, err := f.svc.InterestPaid(ctx, reporting.ReportRequest{Period: must(domain.NewPeriod(date("2026-01-01"), date("2026-12-31")))})
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "interest", interest)
	err = f.store.WithTx(ctx, func(r app.Repos) error {
		cats, err := r.Categories().List(ctx)
		if err != nil {
			return err
		}
		_, err = r.Budgets().Insert(ctx, domain.Budget{
			Name: "Food", Mode: domain.BudgetCap, Amount: money(2_000), Targets: []domain.BudgetTarget{{CategoryID: cats[0].ID}},
			Thresholds: domain.DefaultBudgetThresholds,
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	bva, err := f.svc.BudgetVsActual(ctx, reporting.ReportRequest{})
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "budget", bva)
}

func TestResolveRelative(t *testing.T) {
	today := date("2026-03-15")
	for _, tc := range []struct {
		in       string
		start    int
		from, to string
	}{
		{"this-month", 1, "2026-03-01", "2026-03-31"},
		{"last-month", 1, "2026-02-01", "2026-02-28"},
		{"last-3-months", 1, "2025-12-01", "2026-02-28"},
		{"this-year", 1, "2026-01-01", "2026-12-31"},
		{"last-year", 1, "2025-01-01", "2025-12-31"},
		{"last-month", 27, "2026-01-27", "2026-02-26"},
	} {
		p, err := reporting.ResolveRelative(tc.in, today, tc.start)
		if err != nil {
			t.Fatalf("%s: %v", tc.in, err)
		}
		if p.From() != date(tc.from) || p.To() != date(tc.to) {
			t.Errorf("%s/%d = %s..%s, want %s..%s", tc.in, tc.start, p.From(), p.To(), tc.from, tc.to)
		}
	}
	for _, bad := range []string{"", "last-0-months", "last-x-months", "next-month"} {
		if _, err := reporting.ResolveRelative(bad, today, 1); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestSavedReports(t *testing.T) {
	f := newFx(t)
	seedHistory(t, f)
	ctx := context.Background()
	save := func(name, kind string, spec reporting.SavedSpec) error {
		p, err := f.svc.PlanSave(ctx, name, kind, spec)
		if err != nil {
			return err
		}
		_, err = f.svc.ApplySave(ctx, p)
		return err
	}
	if err := save("Payees", reporting.KindPayees, reporting.SavedSpec{Period: "this-month", Limit: 1}); err != nil {
		t.Fatal(err)
	}
	if err := save("payees", reporting.KindPayees, reporting.SavedSpec{}); !errors.Is(err, app.ErrAlreadyExists) {
		t.Errorf("duplicate name err = %v", err)
	}
	if err := save("x", "nope", reporting.SavedSpec{}); err == nil {
		t.Error("unknown kind accepted")
	}
	if err := save("y", reporting.KindPayees, reporting.SavedSpec{Period: "soon"}); err == nil {
		t.Error("bad period accepted")
	}
	if err := save("NW", reporting.KindNetWorth, reporting.SavedSpec{Period: "last-3-months"}); err != nil {
		t.Fatal(err)
	}
	list, err := f.svc.List(ctx)
	if err != nil || len(list) != 2 || list[0].Name != "NW" {
		t.Fatalf("List = %v, %v", list, err)
	}
	res, err := f.svc.Run(ctx, "pay")
	if err != nil {
		t.Fatal(err)
	}
	pr := res.Data.(reporting.PayeesResult)
	if len(pr.Rows) != 1 || pr.Rows[0].Payee != "Grocer" || pr.Period != "2026-03-01..2026-03-31" {
		t.Fatalf("run = %+v", pr)
	}
	nw, err := f.svc.Run(ctx, "NW")
	if err != nil {
		t.Fatal(err)
	}
	if got := len(nw.Data.(reporting.SeriesResult).Points); got != 3 {
		t.Fatalf("net worth points = %d, want 3", got)
	}
	rm, err := f.svc.PlanRemove(ctx, "NW")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.ApplyRemove(ctx, rm); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Run(ctx, "NW"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("run removed err = %v", err)
	}
}

func TestNetWorthConvertsForeignAccounts(t *testing.T) {
	f := newFx(t)
	ctx := context.Background()
	usd := must(domain.ParseCurrency("USD"))
	err := f.store.WithTx(ctx, func(r app.Repos) error {
		_, err := r.Accounts().Insert(ctx, domain.Account{
			Name: "Dollars", Kind: domain.AccountCash, Currency: usd, OpeningBalance: must(domain.NewMoney(10_000, usd)), OpenedOn: date("2025-12-01"),
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.NetWorth(ctx, reporting.ReportRequest{})
	if !errors.Is(err, app.ErrRateMissing) {
		t.Fatalf("err = %v, want rate_missing", err)
	}
	err = f.store.WithTx(ctx, func(r app.Repos) error {
		return r.FXRates().Upsert(ctx, must(domain.NewFXRate(usd, date("2026-01-01"), must(domain.NewRate(domain.RateScale/2)))))
	})
	if err != nil {
		t.Fatal(err)
	}
	nw, err := f.svc.NetWorth(ctx, reporting.ReportRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if got := nw.Points[0].Amount.Minor; got != 165_000 {
		t.Fatalf("net worth = %d, want 165000", got)
	}
}

func TestSavedRunEveryKind(t *testing.T) {
	f := newFx(t, module.Loans, module.Budget)
	seedHistory(t, f)
	ctx := context.Background()
	ls := lending.New(f.store, clock.NewFixed(date("2026-03-15").In(time.UTC)), noBackup{}, lending.Config{Location: time.UTC})
	plan, err := ls.PlanAdd(ctx, f.cfg.Modules, lending.AddRequest{
		Name: "Mortgage", Kind: "mortgage", Principal: "12000", Disbursed: "2026-01-01", TermMonths: 12, RateType: "fixed", Rate: "3,5", Debit: "Main",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ls.ApplyAdd(ctx, plan); err != nil {
		t.Fatal(err)
	}
	ev, err := ls.PlanEvent(ctx, f.cfg.Modules, lending.EventRequest{Loan: "Mortgage", Kind: "rate_change", Date: "2026-02-01", Rate: "2"})
	if err != nil {
		t.Fatal(err)
	}
	if err := ls.ApplyEvent(ctx, ev); err != nil {
		t.Fatal(err)
	}
	for kind, spec := range map[string]reporting.SavedSpec{
		reporting.KindBalance:  {Account: "Main", Period: "last-2-months"},
		reporting.KindBudget:   {Period: "last-month"},
		reporting.KindDebt:     {Loan: "Mortgage", Period: "this-year"},
		reporting.KindInterest: {Period: "this-year"},
		reporting.KindNetWorth: {},
		reporting.KindPayees:   {},
		reporting.KindForecast: {MinBalance: "1000"},
		reporting.KindSpending: {Period: "this-month", Account: "Main"},
		reporting.KindCashFlow: {Period: "last-3-months"},
	} {
		p, err := f.svc.PlanSave(ctx, kind, kind, spec)
		if err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		if _, err := f.svc.ApplySave(ctx, p); err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		if _, err := f.svc.Run(ctx, kind); err != nil {
			t.Errorf("run %s: %v", kind, err)
		}
	}
	interest, err := f.svc.InterestPaid(ctx, reporting.ReportRequest{Loan: "mort", Period: must(domain.NewPeriod(date("2026-01-01"), date("2026-12-31")))})
	if err != nil {
		t.Fatal(err)
	}
	// Only the installments due up to today (Jan 1.. Mar 15) count: Feb 1 and Mar 1 under the new rate, plus none else.
	if len(interest.Rows) != 1 || interest.Rows[0].Year != 2026 || interest.Rows[0].Interest.Minor <= 0 {
		t.Fatalf("interest = %+v", interest)
	}
}

// memSource implements reporting.Source over a memstore, as the SQLite store does.
type memSource struct{ st *memstore.Store }

func (m memSource) read(fn func(app.Repos) error) error {
	return m.st.Read(context.Background(), fn)
}

func (m memSource) ListAccounts(_ context.Context, f reporting.AccountFilter) ([]domain.Account, error) {
	var out []domain.Account
	err := m.read(func(r app.Repos) error {
		all, err := r.Accounts().List(context.Background())
		for _, a := range all {
			if (f.IncludeArchived || !a.Archived) && (len(f.AccountIDs) == 0 || slices.Contains(f.AccountIDs, a.ID)) {
				out = append(out, a)
			}
		}
		return err
	})
	return out, err
}

func (m memSource) ListCategories(_ context.Context, ids []domain.CategoryID) ([]domain.Category, error) {
	var out []domain.Category
	err := m.read(func(r app.Repos) error {
		all, err := r.Categories().List(context.Background())
		for _, c := range all {
			if slices.Contains(ids, c.ID) {
				out = append(out, c)
			}
		}
		return err
	})
	return out, err
}

func (m memSource) ListTransactions(_ context.Context, f reporting.TransactionFilter) ([]domain.Transaction, error) {
	var out []domain.Transaction
	err := m.read(func(r app.Repos) error {
		tf, err := domain.NewTxFilter(domain.TxFilterSpec{From: f.Period.From(), To: f.Period.To(), AccountIDs: f.AccountIDs})
		if err != nil {
			return err
		}
		out, err = r.Transactions().List(context.Background(), tf, nil, 0)
		return err
	})
	return out, err
}

func (m memSource) BalanceAt(context.Context, domain.AccountID, domain.Date) (domain.Money, error) {
	return domain.Money{}, errors.New("unused")
}

func (m memSource) FXRateOnOrBefore(context.Context, domain.Currency, domain.Currency, domain.Date) (domain.Rate, error) {
	return 0, domain.ErrNotFound
}

func TestSavedMVPReports(t *testing.T) {
	f := newFx(t)
	seedHistory(t, f)
	ctx := context.Background()
	for kind, spec := range map[string]reporting.SavedSpec{
		reporting.KindSpending: {Period: "this-month", Account: "Main"},
		reporting.KindCashFlow: {Period: "last-3-months"},
	} {
		p, err := f.svc.PlanSave(ctx, kind, kind, spec)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.svc.ApplySave(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	sp, err := f.svc.Run(ctx, reporting.KindSpending)
	if err != nil {
		t.Fatal(err)
	}
	if got := sp.Data.(reporting.SpendingResult).Total.Minor(); got != 2_000 {
		t.Fatalf("spending total = %d, want 2000", got)
	}
	cf, err := f.svc.Run(ctx, reporting.KindCashFlow)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(cf.Data.(reporting.CashFlowResult).Months); got != 3 {
		t.Fatalf("cash-flow months = %d, want 3", got)
	}
	noSrc := reporting.NewAnalytics(f.store, clock.NewFixed(date("2026-03-15").In(time.UTC)), nil, f.cfg)
	if _, err := noSrc.Run(ctx, reporting.KindSpending); !errors.Is(err, domain.ErrMissingField) {
		t.Fatalf("run without source err = %v", err)
	}
}

func TestForecastIgnoresLoanInterestHistory(t *testing.T) {
	f := newFx(t)
	seedHistory(t, f)
	ctx := context.Background()
	base, err := f.svc.Forecast(ctx, reporting.ForecastRequest{HorizonDays: 30})
	if err != nil {
		t.Fatal(err)
	}
	var cat domain.Category
	err = f.store.WithTx(ctx, func(r app.Repos) (err error) {
		cat, err = r.Categories().Insert(ctx, domain.Category{Name: lending.InterestCategory, Kind: domain.CategoryExpense})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	// An interest expense booked when an installment was confirmed, and a stray one by category only.
	f.tx(t, f.main, "2026-02-20", -4_000, func(x *domain.Transaction) { x.Kind = domain.KindInterest; x.CategoryID = cat.ID })
	f.tx(t, f.main, "2026-01-20", -2_000, func(x *domain.Transaction) { x.CategoryID = cat.ID })
	with, err := f.svc.Forecast(ctx, reporting.ForecastRequest{HorizonDays: 30})
	if err != nil {
		t.Fatal(err)
	}
	// The history gained 6000 of expenses, but they must not move the projection; the opening
	// balance did move by -6000 (both rows are in the past), so compare the daily slope.
	slope := func(r reporting.ForecastResult) int64 { return r.Total[0].Amount.Minor - r.Total[30].Amount.Minor }
	if slope(base) != slope(with) {
		t.Fatalf("loan interest leaked into the variable estimate: %d vs %d", slope(base), slope(with))
	}
}

func TestForecastMinimumBalance(t *testing.T) {
	f := newFx(t)
	seedHistory(t, f)
	ctx := context.Background()
	// Main holds 289000, spends ~1000/month on average; 288990 is crossed on the first days.
	res, err := f.svc.Forecast(ctx, reporting.ForecastRequest{HorizonDays: 30, MinBalance: "2889.50"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Accounts[0].FirstBelowMin == "" || res.Accounts[0].FirstBelowZero != "" {
		t.Fatalf("main = %+v", res.Accounts[0])
	}
	if res.Accounts[1].FirstBelowMin == "" { // Savings 500.00 < 2889.50 from day zero
		t.Fatalf("savings = %+v", res.Accounts[1])
	}
	hi, err := f.svc.Forecast(ctx, reporting.ForecastRequest{HorizonDays: 30, MinBalance: "4000"})
	if err != nil {
		t.Fatal(err)
	}
	if hi.FirstBelowMin != "2026-03-15" || hi.FirstBelowZero != "" {
		t.Fatalf("total below min = %q, below zero = %q", hi.FirstBelowMin, hi.FirstBelowZero)
	}
	none, err := f.svc.Forecast(ctx, reporting.ForecastRequest{HorizonDays: 30})
	if err != nil {
		t.Fatal(err)
	}
	if none.FirstBelowMin != "" || none.Accounts[0].FirstBelowMin != "" {
		t.Fatalf("minimum reported without being set: %+v", none)
	}
	if _, err := f.svc.Forecast(ctx, reporting.ForecastRequest{MinBalance: "abc"}); err == nil {
		t.Fatal("bad minimum accepted")
	}
}
