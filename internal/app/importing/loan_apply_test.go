package importing_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"pgregory.net/rapid"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
	"github.com/0xballo/fui/pkg/fuiproto"
)

func (e env) storedLoans(t tb) []domain.Loan {
	t.Helper()
	var out []domain.Loan
	err := e.store.Read(context.Background(), func(r app.Repos) (err error) {
		out, err = r.Loans().List(context.Background())
		return err
	})
	if err != nil {
		t.Fatalf("list loans: %v", err)
	}
	return out
}

func (e env) scheduleAt(t tb, id domain.LoanID, at string) domain.Schedule {
	t.Helper()
	var s domain.Schedule
	err := e.store.Read(context.Background(), func(r app.Repos) (err error) {
		s, err = r.Loans().ScheduleAt(context.Background(), id, date(at))
		return err
	})
	if err != nil {
		t.Fatalf("schedule at %s: %v", at, err)
	}
	return s
}

func (e env) events(t tb, id domain.LoanID) []domain.LoanEvent {
	t.Helper()
	var out []domain.LoanEvent
	err := e.store.Read(context.Background(), func(r app.Repos) (err error) {
		out, err = r.Loans().ListEvents(context.Background(), id)
		return err
	})
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	return out
}

func newLoanStream() []fuiproto.Record {
	return []fuiproto.Record{
		account("r1", "Bank", "EUR"), loanAccount("l1", "Home"),
		scheduleRec("l1", "2026-01-01", 300, withLoanMeta("r1")),
		instRec("l1", "2026-02-01", 1000, 5, 1000), instRec("l1", "2026-03-01", 1000, 2, 0),
	}
}

func TestApplyLoans(t *testing.T) {
	ctx := context.Background()
	mapMortgage := func(r *importing.PlanRequest) { r.Mappings = map[string]string{"l1": "Mortgage"} }

	t.Run("new loan and version in a loan-only file, re-import writes nothing", func(t *testing.T) {
		e := newEnv(nil)
		src := stream("fui", "", newLoanStream()...)
		res := must(e.svc.ApplyImport(ctx, e.plan(t, src, createAccounts, loansOn)))
		if res.Loans != 1 || res.LoanVersions != 1 || res.Transactions != 0 || res.Accounts != 2 || len(res.Warnings) != 0 {
			t.Fatalf("result = %+v", res)
		}
		loans := e.storedLoans(t)
		if len(loans) != 1 || loans[0].Name != "Home" || loans[0].Principal != money(2000) {
			t.Fatalf("loans = %+v", loans)
		}
		l := loans[0]
		if l.LiabilityAccountID <= 0 || l.DebitAccount <= 0 || l.LiabilityAccountID == l.DebitAccount {
			t.Fatalf("account ids = %d, %d", l.LiabilityAccountID, l.DebitAccount)
		}
		s := e.scheduleAt(t, l.ID, "2026-01-01")
		if s.Origin != domain.ScheduleOriginImported || len(s.Installments) != 2 || s.Rate != 300 {
			t.Fatalf("schedule = %+v", s)
		}

		before := snapshot(e.store)
		p := e.plan(t, src, createAccounts, loansOn)
		res = must(e.svc.ApplyImport(ctx, p))
		if res.Loans != 0 || res.LoanVersions != 0 || res.Batch.ID != 0 || len(res.Warnings) != 0 {
			t.Fatalf("re-import result = %+v", res)
		}
		if snapshot(e.store) != before {
			t.Fatal("re-import changed the store")
		}
	})

	t.Run("renegotiation of an existing loan", func(t *testing.T) {
		e := newEnv(nil)
		loan := e.seedLoan(t)
		recs := []fuiproto.Record{
			loanAccount("l1", "Mortgage"), scheduleRec("l1", "2026-02-15", 250), instRec("l1", "2026-03-01", 1000, 2, 0),
		}
		res := must(e.svc.ApplyImport(ctx, e.plan(t, stream("fui", "", recs...), loansOn, mapMortgage)))
		if res.Loans != 0 || res.LoanVersions != 1 {
			t.Fatalf("result = %+v", res)
		}
		if s := e.scheduleAt(t, loan.ID, "2026-02-20"); s.Rate != 250 || len(s.Installments) != 1 {
			t.Fatalf("schedule = %+v", s)
		}
		if evs := e.events(t, loan.ID); len(evs) != 1 || evs[0].Kind != domain.EventKindRateChange {
			t.Fatalf("events = %+v", evs)
		}
	})

	t.Run("renegotiation in the stream of the new loan uses the created loan", func(t *testing.T) {
		e := newEnv(nil)
		recs := append(newLoanStream(), scheduleRec("l1", "2026-02-15", 200), instRec("l1", "2026-03-01", 1000, 1, 0))
		res := must(e.svc.ApplyImport(ctx, e.plan(t, stream("fui", "", recs...), createAccounts, loansOn)))
		if res.Loans != 1 || res.LoanVersions != 2 || len(res.Warnings) != 0 {
			t.Fatalf("result = %+v", res)
		}
	})

	t.Run("module off and not enabled", func(t *testing.T) {
		e := newEnv(nil)
		res := must(e.svc.ApplyImport(ctx, e.plan(t, stream("fui", "", newLoanStream()...), createAccounts)))
		if res.Loans != 0 || res.LoanVersions != 0 || len(e.storedLoans(t)) != 0 {
			t.Fatalf("result = %+v", res)
		}
		if got := warnings(importing.ImportPlan{Warnings: res.Warnings}); !strings.Contains(got, "record of module loans not imported") {
			t.Fatalf("warnings = %q", got)
		}
	})

	t.Run("module off but enabled by the import", func(t *testing.T) {
		e := newEnv(nil)
		w := &fakeModuleWriter{}
		plan := e.plan(t, stream("fui", "", newLoanStream()...), createAccounts)
		res := must(e.svc.ApplyImportWith(ctx, plan, importing.ApplyOptions{
			EnableModules: []module.ID{module.Loans}, Modules: module.Default(), ModuleWriter: w,
		}))
		if res.Loans != 1 || res.LoanVersions != 1 || len(res.Warnings) != 0 || len(e.storedLoans(t)) != 1 {
			t.Fatalf("result = %+v", res)
		}
	})

	t.Run("rejected version warns and the rest of the batch is imported", func(t *testing.T) {
		e := newEnv(nil)
		e.seedLoan(t)
		recs := []fuiproto.Record{
			account("r1", "Bank", "EUR"), txRec("r1", "c", "2026-10-01", -100, "coffee"),
			loanAccount("l1", "Mortgage"), scheduleRec("l1", "2026-02-15", 250), instRec("l1", "2026-03-01", 1500, 2, 0),
		}
		res := must(e.svc.ApplyImport(ctx, e.plan(t, stream("fui", "", recs...), createAccounts, loansOn, mapMortgage)))
		if res.Transactions != 1 || res.LoanVersions != 0 || len(res.Warnings) != 1 || res.Warnings[0].Line == 0 {
			t.Fatalf("result = %+v", res)
		}
		if !strings.Contains(res.Warnings[0].Message, "principal sums to") {
			t.Fatalf("warning = %q", res.Warnings[0].Message)
		}
	})

	t.Run("rejected new loan is not left behind", func(t *testing.T) {
		e := newEnv(nil)
		recs := newLoanStream()
		recs[4].Installment.Remaining = 5 // final residual must be zero
		recs = append(recs, scheduleRec("l1", "2026-02-15", 200), instRec("l1", "2026-03-01", 1000, 1, 0))
		res := must(e.svc.ApplyImport(ctx, e.plan(t, stream("fui", "", recs...), createAccounts, loansOn)))
		if res.Loans != 0 || res.LoanVersions != 0 || len(e.storedLoans(t)) != 0 || len(res.Warnings) != 2 {
			t.Fatalf("result = %+v loans = %+v", res, e.storedLoans(t))
		}
	})
}

// TestApplyRenegotiationProperty checks that an imported renegotiation applied through
// ApplyImport leaves a version whose principal adds up to the capital and ends at zero.
func TestApplyRenegotiationProperty(t *testing.T) {
	split := func(t *rapid.T, label string, total int64, n int) []int64 {
		parts := make([]int64, n)
		left := total
		for i := range parts {
			if i == n-1 {
				parts[i] = left
				break
			}
			parts[i] = rapid.Int64Range(1, left-int64(n-1-i)).Draw(t, fmt.Sprintf("%s%d", label, i))
			left -= parts[i]
		}
		return parts
	}
	records := func(parts []int64) []fuiproto.Record {
		var recs []fuiproto.Record
		var total int64
		for _, p := range parts {
			total += p
		}
		for i, p := range parts {
			total -= p
			recs = append(recs, instRec("l1", fmt.Sprintf("2026-%02d-01", i+2), p, 1, total))
		}
		return recs
	}
	rapid.Check(t, func(t *rapid.T) {
		capital := rapid.Int64Range(6, 100000).Draw(t, "capital")
		first := split(t, "a", capital, rapid.IntRange(1, 6).Draw(t, "n"))
		second := split(t, "b", capital, rapid.IntRange(1, 6).Draw(t, "m"))

		recs := []fuiproto.Record{
			account("r1", "Bank", "EUR"), loanAccount("l1", "Home"),
			scheduleRec("l1", "2026-01-01", 300, withLoanMeta("r1"), func(s *fuiproto.ScheduleRecord) {
				s.Principal, s.TermMonths = capital, len(first)
			}),
		}
		recs = append(recs, records(first)...)
		recs = append(recs, scheduleRec("l1", "2026-01-15", 250))
		recs = append(recs, records(second)...)

		e := newEnv(nil)
		res := must(e.svc.ApplyImport(context.Background(), e.plan(t, stream("fui", "", recs...), createAccounts, loansOn)))
		if res.Loans != 1 || res.LoanVersions != 2 || len(res.Warnings) != 0 {
			t.Fatalf("result = %+v", res)
		}
		loans := e.storedLoans(t)
		s := e.scheduleAt(t, loans[0].ID, "2026-01-20")
		var sum int64
		for _, in := range s.Installments {
			sum += in.Principal.Minor()
		}
		if sum != capital || s.Installments[len(s.Installments)-1].Remaining.Minor() != 0 {
			t.Fatalf("principal sum = %d (capital %d), schedule = %+v", sum, capital, s)
		}
	})
}
