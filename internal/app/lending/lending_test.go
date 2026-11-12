package lending_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"pgregory.net/rapid"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/lending"
	"github.com/0xballo/fui/internal/clock"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
	"github.com/0xballo/fui/internal/testutil/memstore"
)

// tb is the part of testing.T and rapid.T the helpers use.
type tb interface {
	Helper()
	Fatalf(string, ...any)
	Fatal(...any)
}

type spyBackup struct{ n int }

func (b *spyBackup) Backup(context.Context, string) (string, error) { b.n++; return "b", nil }
func (b *spyBackup) Restore(context.Context, string) error          { return nil }

type env struct {
	svc    *lending.Service
	store  *memstore.Store
	backup *spyBackup
	main   domain.Account
	on     module.Set
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func date(s string) domain.Date { return must(domain.ParseDate(s)) }

func eur() domain.Currency { return must(domain.ParseCurrency("EUR")) }

func money(minor int64) domain.Money { return must(domain.NewMoney(minor, eur())) }

func newEnv(t tb) env {
	t.Helper()
	e := env{
		store: memstore.New(), backup: &spyBackup{},
		on: must(module.Validate([]module.ID{module.Core, module.Recurring, module.Loans})),
	}
	e.svc = lending.New(e.store, clock.NewFixed(date("2026-01-15").In(time.UTC)), e.backup, lending.Config{})
	ctx := context.Background()
	err := e.store.WithTx(ctx, func(r app.Repos) (err error) {
		e.main, err = r.Accounts().Insert(ctx, domain.Account{
			Name: "Main", Kind: domain.AccountChecking, Currency: eur(), OpeningBalance: money(5_000_000),
			OpenedOn: date("2025-01-01"),
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func addReq() lending.AddRequest {
	return lending.AddRequest{
		Name: "Mortgage", Kind: "mortgage", Principal: "12000", Disbursed: "2026-01-01", TermMonths: 12,
		RateType: "fixed", Rate: "3,5", Debit: "Main",
	}
}

func (e env) add(t tb, req lending.AddRequest) lending.Info {
	t.Helper()
	ctx := context.Background()
	p, err := e.svc.PlanAdd(ctx, e.on, req)
	if err != nil {
		t.Fatalf("PlanAdd: %v", err)
	}
	info, err := e.svc.ApplyAdd(ctx, p)
	if err != nil {
		t.Fatalf("ApplyAdd: %v", err)
	}
	return info
}

func (e env) netWorth(t tb, at domain.Date) int64 {
	t.Helper()
	var sum int64
	err := e.store.Read(context.Background(), func(r app.Repos) error {
		accts, err := r.Accounts().List(context.Background())
		if err != nil {
			return err
		}
		for _, a := range accts {
			s, err := r.Transactions().SumUpTo(context.Background(), a.ID, at)
			if err != nil {
				return err
			}
			sum += a.OpeningBalance.Minor() + s
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return sum
}

func (e env) generate(t tb, until string) int {
	t.Helper()
	p, err := e.svc.PlanInstallments(context.Background(), e.on, "", until)
	if err != nil {
		t.Fatalf("PlanInstallments: %v", err)
	}
	n, err := e.svc.ApplyInstallments(context.Background(), p)
	if err != nil {
		t.Fatalf("ApplyInstallments: %v", err)
	}
	return n
}

func (e env) pending(t tb) []domain.Transaction {
	t.Helper()
	f := must(domain.NewTxFilter(domain.TxFilterSpec{Statuses: []domain.TxStatus{domain.StatusPending}, Sort: domain.TxSortDate}))
	var out []domain.Transaction
	err := e.store.Read(context.Background(), func(r app.Repos) (err error) {
		out, err = r.Transactions().List(context.Background(), f, nil, 0)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestModuleGate(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	off := must(module.Validate([]module.ID{module.Core}))
	calls := map[string]func() error{
		"add":          func() error { _, err := e.svc.PlanAdd(ctx, off, addReq()); return err },
		"list":         func() error { _, err := e.svc.List(ctx, off); return err },
		"show":         func() error { _, err := e.svc.Show(ctx, off, "x", ""); return err },
		"event":        func() error { _, err := e.svc.PlanEvent(ctx, off, lending.EventRequest{}); return err },
		"installments": func() error { _, err := e.svc.PlanInstallments(ctx, off, "", ""); return err },
		"confirm":      func() error { _, err := e.svc.PlanConfirm(ctx, off, 1); return err },
	}
	for name, call := range calls {
		if err := call(); !errors.Is(err, app.ErrModuleDisabled) {
			t.Errorf("%s: err = %v, want module_disabled", name, err)
		}
	}
}

func TestAddAndShow(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	info := e.add(t, addReq())
	if info.Account.Kind != domain.AccountLoan || info.Account.OpeningBalance.Minor() != -1_200_000 {
		t.Fatalf("liability account = %+v", info.Account)
	}
	if info.Loan.Name != "Mortgage" || info.Loan.LiabilityAccountID != info.Account.ID || info.Account.Name != "Mortgage" {
		t.Fatalf("loan identity = %+v / %+v", info.Loan, info.Account)
	}
	v, err := e.svc.Show(ctx, e.on, "mort", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Schedule.Installments) != 12 || v.Schedule.Origin != domain.ScheduleOriginComputed || v.Schedule.Rate != 350 {
		t.Errorf("schedule = %+v", v.Schedule)
	}
	list, err := e.svc.List(ctx, e.on)
	if err != nil || len(list) != 1 || list[0].Loan.Name != "Mortgage" {
		t.Errorf("List = %+v, %v", list, err)
	}
	if _, err := e.svc.PlanAdd(ctx, e.on, addReq()); !errors.Is(err, app.ErrAlreadyExists) {
		t.Errorf("duplicate name err = %v", err)
	}
}

func TestAddValidation(t *testing.T) {
	e := newEnv(t)
	bank := func(r *lending.AddRequest) { r.Installment = "1020" }
	imp := func(r *lending.AddRequest) {
		r.Imported = []domain.Installment{{Due: date("2026-02-01"), Principal: money(1_000_000), Remaining: money(200_000)}}
	}
	tests := []struct {
		name string
		mut  func(*lending.AddRequest)
		ok   bool
	}{
		{"bank installment", bank, true},
		{"bad rate decimals", func(r *lending.AddRequest) { r.Rate = "3,555" }, false},
		{"bad kind", func(r *lending.AddRequest) { r.Kind = "x" }, false},
		{"bad debit", func(r *lending.AddRequest) { r.Debit = "Nope" }, false},
		{"imported not repaying", imp, false},
		{"bad principal", func(r *lending.AddRequest) { r.Principal = "x" }, false},
		{"bad date", func(r *lending.AddRequest) { r.Disbursed = "zzz" }, false},
		{"bad rate type", func(r *lending.AddRequest) { r.RateType = "x" }, false},
		{"bad bank installment", func(r *lending.AddRequest) { r.Installment = "x" }, false},
		{"zero term", func(r *lending.AddRequest) { r.TermMonths = 0 }, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := addReq()
			tc.mut(&req)
			_, err := e.svc.PlanAdd(context.Background(), e.on, req)
			if (err == nil) != tc.ok {
				t.Errorf("err = %v, want ok=%v", err, tc.ok)
			}
		})
	}
}

func TestAddImportedSchedule(t *testing.T) {
	e := newEnv(t)
	req := addReq()
	req.Principal, req.TermMonths = "200", 2
	req.Imported = []domain.Installment{
		{Due: date("2026-02-01"), Principal: money(10_000), Interest: money(500), Remaining: money(10_000)},
		{Due: date("2026-03-01"), Principal: money(10_000), Interest: money(250), Remaining: money(0)},
	}
	e.add(t, req)
	v, err := e.svc.Show(context.Background(), e.on, "Mortgage", "")
	if err != nil || v.Schedule.Origin != domain.ScheduleOriginImported || len(v.Schedule.Installments) != 2 {
		t.Errorf("Show = %+v, %v", v.Schedule, err)
	}
}

func TestEventVersionsSchedule(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.add(t, addReq())
	p, err := e.svc.PlanEvent(ctx, e.on, lending.EventRequest{Loan: "Mortgage", Kind: "rate_change", Date: "2026-07-01", Rate: "5"})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.svc.ApplyEvent(ctx, p); err != nil {
		t.Fatal(err)
	}
	old, err := e.svc.Show(ctx, e.on, "Mortgage", "2026-06-30")
	if err != nil {
		t.Fatal(err)
	}
	cur, err := e.svc.Show(ctx, e.on, "Mortgage", "2026-07-01")
	if err != nil {
		t.Fatal(err)
	}
	if old.Schedule.Rate != 350 || cur.Schedule.Rate != 500 || cur.Schedule.ValidFrom != date("2026-07-01") || len(cur.Events) != 1 {
		t.Errorf("old=%+v cur=%+v", old.Schedule.Rate, cur.Schedule)
	}
	if got := cur.Schedule.Installments[len(cur.Schedule.Installments)-1].Remaining.Minor(); got != 0 {
		t.Errorf("final residual = %d", got)
	}
	if _, err := e.svc.PlanEvent(ctx, e.on, lending.EventRequest{Loan: "Mortgage", Kind: "rate_change", Date: "2026-07-01", Rate: "4"}); !errors.Is(err, app.ErrAlreadyExists) {
		t.Errorf("same-date event err = %v", err)
	}
	for _, bad := range []lending.EventRequest{
		{Loan: "Mortgage", Kind: "bogus", Date: "2026-08-01"},
		{Loan: "Mortgage", Kind: "prepayment", Date: "2026-08-01", Prepaid: "100", Mode: "bogus"},
		{Loan: "Nope", Kind: "rate_change", Date: "2026-08-01", Rate: "4"},
	} {
		if _, err := e.svc.PlanEvent(ctx, e.on, bad); err == nil {
			t.Errorf("PlanEvent(%+v) succeeded", bad)
		}
	}
}

func TestPrepaymentEvent(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.add(t, addReq())
	p, err := e.svc.PlanEvent(ctx, e.on, lending.EventRequest{
		Loan: "Mortgage", Kind: "prepayment", Date: "2026-06-15", Prepaid: "2000", Mode: "reduce_term",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.svc.ApplyEvent(ctx, p); err != nil {
		t.Fatal(err)
	}
	if len(p.Schedule.Installments) >= 7 {
		t.Errorf("reduce_term kept %d installments", len(p.Schedule.Installments))
	}
}

func TestInstallmentsIdempotentAcrossVersions(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.add(t, addReq())
	if n := e.generate(t, "2026-04-01"); n != 3 {
		t.Fatalf("first run created %d, want 3", n)
	}
	if n := e.generate(t, "2026-04-01"); n != 0 {
		t.Fatalf("second run created %d, want 0", n)
	}
	p, err := e.svc.PlanEvent(ctx, e.on, lending.EventRequest{Loan: "Mortgage", Kind: "rate_change", Date: "2026-04-15", Rate: "6"})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.svc.ApplyEvent(ctx, p); err != nil {
		t.Fatal(err)
	}
	if n := e.generate(t, "2026-12-31"); n != 8 {
		t.Fatalf("run through the end created %d, want 8", n)
	}
	if e.backup.n != 2 {
		t.Errorf("backups = %d, want 2 (bulk runs)", e.backup.n)
	}
	for _, tx := range e.pending(t) {
		if tx.Description != "Mortgage installment" || tx.Amount.Sign() >= 0 || tx.AccountID != e.main.ID {
			t.Errorf("pending = %+v", tx)
		}
	}
}

func (e env) confirm(t tb, tx domain.Transaction) lending.ConfirmPlan {
	t.Helper()
	p, err := e.svc.PlanConfirm(context.Background(), e.on, tx.ID)
	if err != nil {
		t.Fatalf("PlanConfirm: %v", err)
	}
	if err := e.svc.ApplyConfirm(context.Background(), p); err != nil {
		t.Fatalf("ApplyConfirm: %v", err)
	}
	return p
}

func TestConfirmSplitsInstallment(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.add(t, addReq())
	due := date("2026-02-01")
	before := e.netWorth(t, due)
	e.generate(t, "2026-02-01")
	tx := e.pending(t)[0]
	v, _ := e.svc.Show(ctx, e.on, "Mortgage", "")
	inst := v.Schedule.Installments[0]
	p := e.confirm(t, tx)
	if len(p.Warnings) != 0 || p.ProposedRate != 0 {
		t.Errorf("unexpected warnings: %+v", p.Warnings)
	}
	if got := before - e.netWorth(t, due); got != inst.Interest.Minor()+inst.Fees.Minor() {
		t.Errorf("net worth fell by %d, want interest+fees %d", got, inst.Interest.Minor()+inst.Fees.Minor())
	}
	if len(e.pending(t)) != 0 {
		t.Errorf("still pending: %+v", e.pending(t))
	}
	var loanBal int64
	_ = e.store.Read(ctx, func(r app.Repos) error {
		s, err := r.Transactions().SumUpTo(ctx, v.Account.ID, due)
		loanBal = v.Account.OpeningBalance.Minor() + s
		return err
	})
	if loanBal != -inst.Remaining.Minor() {
		t.Errorf("loan balance = %d, want -%d", loanBal, inst.Remaining.Minor())
	}
	// second installment reuses the interest category
	e.generate(t, "2026-03-01")
	e.confirm(t, e.pending(t)[0])
	_ = e.store.Read(ctx, func(r app.Repos) error {
		cats, _ := r.Categories().List(ctx)
		n := 0
		for _, c := range cats {
			if c.Name == lending.InterestCategory {
				n++
			}
		}
		if n != 1 {
			t.Errorf("%d interest categories", n)
		}
		return nil
	})
}

func TestConfirmMismatchProposesRate(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.add(t, addReq())
	e.generate(t, "2026-02-01")
	tx := e.pending(t)[0]
	// the bank debited more than scheduled: the rate went up
	tx.Amount = money(tx.Amount.Minor() - 3_000)
	if err := e.store.WithTx(ctx, func(r app.Repos) error { return r.Transactions().Update(ctx, tx) }); err != nil {
		t.Fatal(err)
	}
	p, err := e.svc.PlanConfirm(ctx, e.on, tx.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Warnings) != 1 || p.ProposedRate <= 350 {
		t.Fatalf("warnings=%v proposed=%v, want a rate above 3.50%%", p.Warnings, p.ProposedRate)
	}
	if err := e.svc.ApplyConfirm(ctx, p); err != nil {
		t.Fatal(err)
	}
	v, _ := e.svc.Show(ctx, e.on, "Mortgage", "")
	if v.Schedule.Rate != 350 || len(v.Events) != 0 {
		t.Errorf("rate was changed: %+v", v.Schedule.Rate)
	}
}

func TestConfirmRejects(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.add(t, addReq())
	var plain domain.Transaction
	_ = e.store.WithTx(ctx, func(r app.Repos) (err error) {
		plain, err = r.Transactions().Insert(ctx, must(domain.NewTransaction(domain.Transaction{
			AccountID: e.main.ID, Date: date("2026-02-01"), Amount: money(-100), Status: domain.StatusPending,
			Kind: domain.KindPayment, Description: "Other",
		})))
		return err
	})
	if _, err := e.svc.PlanConfirm(ctx, e.on, plain.ID); err == nil {
		t.Error("confirmed a transaction that is not an installment")
	}
	e.generate(t, "2026-02-01")
	e.confirm(t, e.pending(t)[1])
	if _, err := e.svc.PlanConfirm(ctx, e.on, 999); err == nil {
		t.Error("confirmed a missing transaction")
	}
}

// Confirming an installment lowers net worth by exactly interest+fees, whatever the loan.
func TestConfirmNetWorthProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		e := newEnv(t)
		req := addReq()
		req.Principal = rapid.StringMatching(`[1-9][0-9]{3,5}`).Draw(t, "principal")
		req.TermMonths = rapid.IntRange(1, 60).Draw(t, "term")
		req.Rate = rapid.SampledFrom([]string{"0,5", "2,75", "4", "9,99"}).Draw(t, "rate")
		e.add(t, req)
		due := date("2026-02-01")
		before := e.netWorth(t, due)
		e.generate(t, "2026-02-01")
		tx := e.pending(t)[0]
		v, err := e.svc.Show(context.Background(), e.on, "Mortgage", "")
		if err != nil {
			t.Fatal(err)
		}
		inst := v.Schedule.Installments[0]
		e.confirm(t, tx)
		if got := before - e.netWorth(t, due); got != inst.Interest.Minor()+inst.Fees.Minor() {
			t.Fatalf("net worth fell by %d, want %d", got, inst.Interest.Minor()+inst.Fees.Minor())
		}
	})
}

func TestInterestOnlyAndUnderpaidInstallments(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	req := addReq()
	req.Principal, req.TermMonths = "200", 2
	req.Imported = []domain.Installment{
		{Due: date("2026-02-01"), Interest: money(500), Remaining: money(20_000)},
		{Due: date("2026-03-01"), Principal: money(20_000), Interest: money(250), Remaining: money(0)},
	}
	e.add(t, req)
	if _, err := e.svc.PlanInstallments(ctx, e.on, "nope", ""); err == nil {
		t.Fatal("unknown loan ref accepted")
	}
	p, err := e.svc.PlanInstallments(ctx, e.on, "Mort", "2026-03-01")
	if err != nil || len(p.Pending) != 2 {
		t.Fatalf("plan = %+v, %v", p.Pending, err)
	}
	if _, err := e.svc.ApplyInstallments(ctx, p); err != nil {
		t.Fatal(err)
	}
	first, second := e.pending(t)[0], e.pending(t)[1]
	e.confirm(t, first) // no principal: the transaction itself becomes the interest expense
	// the bank debited less than the scheduled principal: principal is capped to what was paid
	second.Amount = money(-10_000)
	if err := e.store.WithTx(ctx, func(r app.Repos) error { return r.Transactions().Update(ctx, second) }); err != nil {
		t.Fatal(err)
	}
	plan := e.confirm(t, second)
	if len(plan.Warnings) != 1 || plan.In.Amount.Minor() != 10_000 || !plan.Expense.Amount.Currency().IsZero() {
		t.Errorf("plan = %+v", plan)
	}
	_ = e.store.Read(ctx, func(r app.Repos) error {
		tx, _ := r.Transactions().Get(ctx, first.ID)
		if tx.Kind != domain.KindInterest || tx.Status != domain.StatusBooked || tx.CategoryID == 0 || tx.Amount.Minor() != -500 {
			t.Errorf("interest-only tx = %+v", tx)
		}
		return nil
	})
}

func TestShowBeforeDisbursement(t *testing.T) {
	e := newEnv(t)
	e.add(t, addReq())
	v, err := e.svc.Show(context.Background(), e.on, "Mortgage", "2025-06-01")
	if err != nil || v.Schedule.ValidFrom != date("2026-01-01") {
		t.Errorf("Show = %v, %v", v.Schedule.ValidFrom, err)
	}
	if _, err := e.svc.Show(context.Background(), e.on, "Nope", ""); err == nil {
		t.Error("unknown loan accepted")
	}
}

func TestApplyInstallmentsEmpty(t *testing.T) {
	e := newEnv(t)
	if n, err := e.svc.ApplyInstallments(context.Background(), lending.InstallmentsPlan{}); n != 0 || err != nil {
		t.Errorf("= %d, %v", n, err)
	}
	if _, err := e.svc.PlanInstallments(context.Background(), e.on, "", "zzz"); err == nil {
		t.Error("bad until accepted")
	}
}
