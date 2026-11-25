package importing_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
	"github.com/0xballo/fui/pkg/fuiproto"
)

func loanAccount(ref, name string) fuiproto.Record {
	return account(ref, name, "EUR", func(a *fuiproto.AccountRecord) { a.Kind = fuiproto.AccountKindLoan })
}

func scheduleRec(ref, from string, rate int64, mods ...func(*fuiproto.ScheduleRecord)) fuiproto.Record {
	s := &fuiproto.ScheduleRecord{Account: ref, ValidFrom: from, Rate: rate, RateType: fuiproto.RateTypeFixed}
	for _, m := range mods {
		m(s)
	}
	return fuiproto.Record{Type: fuiproto.RecordTypeSchedule, Schedule: s}
}

func instRec(ref, due string, principal, interest, remaining int64) fuiproto.Record {
	return fuiproto.Record{Type: fuiproto.RecordTypeInstallment, Installment: &fuiproto.InstallmentRecord{
		Account: ref, DueDate: due, Principal: principal, Interest: interest, Remaining: remaining,
	}}
}

func withLoanMeta(debit string) func(*fuiproto.ScheduleRecord) {
	return func(s *fuiproto.ScheduleRecord) {
		s.DebitAccount, s.Kind, s.Principal, s.DisbursedOn, s.TermMonths = debit, fuiproto.LoanKindMortgage, 2000, "2026-01-01", 2
	}
}

func loansOn(r *importing.PlanRequest) {
	r.Modules = must(module.Validate([]module.ID{module.Core, module.Recurring, module.Loans}))
}

// seedLoan inserts a loan on a new loan account, with a bank schedule version at 2026-01-01.
func (e env) seedLoan(t tb) domain.Loan {
	t.Helper()
	debit := e.seedAccount(t, "Current", "", "")
	var loan domain.Loan
	err := e.store.WithTx(context.Background(), func(r app.Repos) error {
		acc, err := r.Accounts().Insert(context.Background(), domain.Account{
			Name: "Mortgage", Kind: domain.AccountLoan, Currency: eur(), OpeningBalance: money(-2000), OpenedOn: date("2026-01-01"),
		})
		if err != nil {
			return err
		}
		loan, err = r.Loans().Insert(context.Background(), domain.Loan{
			Name: "Mortgage", LiabilityAccountID: acc.ID, Kind: domain.LoanKindMortgage, Principal: money(2000),
			DisbursedOn: date("2026-01-01"), TermMonths: 2, RateType: domain.RateTypeFixed, DebitAccount: debit.ID,
		})
		if err != nil {
			return err
		}
		return r.Loans().AddScheduleVersion(context.Background(), domain.Schedule{
			LoanID: loan.ID, ValidFrom: date("2026-01-01"), Origin: domain.ScheduleOriginImported, Rate: 300,
			Installments: []domain.Installment{
				{Due: date("2026-02-01"), Principal: money(1000), Interest: money(5), Fees: money(0), Remaining: money(1000)},
				{Due: date("2026-03-01"), Principal: money(1000), Interest: money(2), Fees: money(0), Remaining: money(0)},
			},
		})
	})
	if err != nil {
		t.Fatalf("seed loan: %v", err)
	}
	return loan
}

func warnings(p importing.ImportPlan) string {
	var parts []string
	for _, w := range p.Warnings {
		parts = append(parts, w.Message)
	}
	return strings.Join(parts, "\n")
}

func TestPlanLoans(t *testing.T) {
	two := []fuiproto.Record{instRec("l1", "2026-02-01", 1000, 5, 1000), instRec("l1", "2026-03-01", 1000, 2, 0)}
	existing := func(e env) { e.seedLoan(t) }

	t.Run("new version on existing loan", func(t *testing.T) {
		e := newEnv(nil)
		loan := e.seedLoan(t)
		recs := append([]fuiproto.Record{loanAccount("l1", "Mortgage"), scheduleRec("l1", "2026-02-15", 250)},
			instRec("l1", "2026-03-01", 1000, 2, 0))
		recs[1].Schedule.RateType = fuiproto.RateTypeVariable
		// Remaining capital is not checked at plan time; the lending service does it on apply.
		p := e.plan(t, stream("fui", "", recs...), loansOn, func(r *importing.PlanRequest) { r.Mappings = map[string]string{"l1": "Mortgage"} })
		if len(p.Loans) != 1 {
			t.Fatalf("loans = %+v, warnings = %q", p.Loans, warnings(p))
		}
		row := p.Loans[0]
		if row.New || !row.Include || row.Unchanged || row.Module != "" || row.Loan.ID != loan.ID || row.Version.LoanID != loan.ID {
			t.Fatalf("row = %+v", row)
		}
		v := row.Version
		if v.ValidFrom != date("2026-02-15") || v.Rate != 250 || v.RateType != domain.RateTypeVariable || len(v.Installments) != 1 {
			t.Fatalf("version = %+v", v)
		}
		if got := v.Installments[0]; got.Principal != money(1000) || got.Interest != money(2) || got.Fees != money(0) || got.Remaining != money(0) {
			t.Fatalf("installment = %+v", got)
		}
		var found bool
		for _, c := range p.Changes {
			if c.Entity == "loan" {
				found = c.Summary == "create/update 1 loans"
			}
		}
		if !found {
			t.Fatalf("changes = %+v", p.Changes)
		}
	})

	t.Run("re-import is unchanged", func(t *testing.T) {
		e := newEnv(nil)
		existing(e)
		recs := append([]fuiproto.Record{loanAccount("l1", "Mortgage"), scheduleRec("l1", "2026-01-01", 300)}, two...)
		p := e.plan(t, stream("fui", "", recs...), loansOn, func(r *importing.PlanRequest) { r.Mappings = map[string]string{"l1": "Mortgage"} })
		if len(p.Loans) != 1 || !p.Loans[0].Unchanged || p.Loans[0].Include {
			t.Fatalf("loans = %+v", p.Loans)
		}
		for _, c := range p.Changes {
			if c.Entity == "loan" {
				t.Fatalf("unexpected loan change %+v", c)
			}
		}
		// Same date, other rate: not a duplicate.
		recs[1].Schedule.Rate = 310
		p = e.plan(t, stream("fui", "", recs...), loansOn, func(r *importing.PlanRequest) { r.Mappings = map[string]string{"l1": "Mortgage"} })
		if p.Loans[0].Unchanged || !p.Loans[0].Include {
			t.Fatalf("changed rate: %+v", p.Loans[0])
		}
	})

	t.Run("new loan with metadata", func(t *testing.T) {
		e := newEnv(nil)
		recs := append([]fuiproto.Record{account("r1", "Bank", "EUR"), loanAccount("l1", "Home"), scheduleRec("l1", "2026-01-01", 300, withLoanMeta("r1"))}, two...)
		p := e.plan(t, stream("fui", "", recs...), createAccounts, loansOn)
		if len(p.Loans) != 1 {
			t.Fatalf("loans = %+v, warnings = %q", p.Loans, warnings(p))
		}
		row := p.Loans[0]
		if !row.New || row.DebitRef != "r1" || row.Loan.Name != "Home" || row.Loan.Principal != money(2000) ||
			row.Loan.Kind != domain.LoanKindMortgage || row.Loan.TermMonths != 2 || row.Loan.DisbursedOn != date("2026-01-01") ||
			row.Loan.LiabilityAccountID >= 0 || row.Loan.DebitAccount >= 0 {
			t.Fatalf("row = %+v", row)
		}
		var got domain.Account
		for _, na := range p.NewAccounts {
			if na.Ref == "l1" {
				got = na.Account
			}
		}
		if got.OpeningBalance != money(-2000) || got.OpenedOn != date("2026-01-01") {
			t.Fatalf("liability account = %+v", got)
		}
	})

	t.Run("debit account by name and renegotiation in the same stream", func(t *testing.T) {
		e := newEnv(nil)
		e.seedAccount(t, "Current", "", "")
		recs := append([]fuiproto.Record{loanAccount("l1", "Home"), scheduleRec("l1", "2026-01-01", 300, withLoanMeta("current"))}, two...)
		recs = append(recs, scheduleRec("l1", "2026-02-15", 200), instRec("l1", "2026-03-01", 1000, 1, 0))
		p := e.plan(t, stream("fui", "", recs...), createAccounts, loansOn)
		if len(p.Loans) != 2 || !p.Loans[0].New || p.Loans[0].DebitRef != "Current" || p.Loans[1].New || p.Loans[1].Loan.Name != "Home" {
			t.Fatalf("loans = %+v, warnings = %q", p.Loans, warnings(p))
		}
	})

	skips := []struct {
		name string
		recs []fuiproto.Record
		mods []func(*importing.PlanRequest)
		want string
	}{
		{
			"no metadata", append([]fuiproto.Record{loanAccount("l1", "Home"), scheduleRec("l1", "2026-01-01", 300)}, two...),
			[]func(*importing.PlanRequest){createAccounts},
			"loan not created: schedule lacks debit_account, kind, principal, disbursed_on, term_months",
		},
		{
			"unresolved account", append([]fuiproto.Record{loanAccount("l1", "Home"), scheduleRec("l1", "2026-01-01", 300, withLoanMeta("l1"))}, two...),
			nil, "account \"l1\" is unresolved",
		},
		{
			"mapped to a non-loan account", append([]fuiproto.Record{loanAccount("l1", "Home"), scheduleRec("l1", "2026-01-01", 300, withLoanMeta("l1"))}, two...),
			[]func(*importing.PlanRequest){func(r *importing.PlanRequest) { r.Mappings = map[string]string{"l1": "Current"} }},
			"not a loan account",
		},
		{
			"unknown debit account", append([]fuiproto.Record{loanAccount("l1", "Home"), scheduleRec("l1", "2026-01-01", 300, withLoanMeta("nope"))}, two...),
			[]func(*importing.PlanRequest){createAccounts},
			"debit account \"nope\" not found",
		},
		{
			"no installments",
			[]fuiproto.Record{loanAccount("l1", "Home"), scheduleRec("l1", "2026-01-01", 300, withLoanMeta("l1"))},
			[]func(*importing.PlanRequest){createAccounts},
			"no installments",
		},
		{
			"negative amount",
			[]fuiproto.Record{loanAccount("l1", "Home"), scheduleRec("l1", "2026-01-01", 300, withLoanMeta("l1")), instRec("l1", "2026-02-01", -5, 0, 0)},
			[]func(*importing.PlanRequest){createAccounts},
			"must not be negative",
		},
		{
			"rate not positive", append([]fuiproto.Record{loanAccount("l1", "Home"), scheduleRec("l1", "2026-01-01", 0, withLoanMeta("l1"))}, two...),
			[]func(*importing.PlanRequest){createAccounts},
			"rate",
		},
	}
	for _, tc := range skips {
		t.Run("skip: "+tc.name, func(t *testing.T) {
			e := newEnv(nil)
			e.seedAccount(t, "Current", "", "")
			p := e.plan(t, stream("fui", "", tc.recs...), append(tc.mods, loansOn)...)
			if len(p.Loans) != 0 || !strings.Contains(warnings(p), tc.want) || len(p.Errors) != 0 {
				t.Fatalf("loans = %+v errors = %+v warnings = %q, want %q", p.Loans, p.Errors, warnings(p), tc.want)
			}
		})
	}

	t.Run("module off tags rows", func(t *testing.T) {
		e := newEnv(nil)
		recs := append([]fuiproto.Record{account("r1", "Bank", "EUR"), loanAccount("l1", "Home"), scheduleRec("l1", "2026-01-01", 300, withLoanMeta("r1"))}, two...)
		p := e.plan(t, stream("fui", "", recs...), createAccounts)
		if len(p.Loans) != 1 || p.Loans[0].Module != module.Loans {
			t.Fatalf("loans = %+v", p.Loans)
		}
		want := []importing.ModuleRecords{{Module: module.Loans, Count: 3, Kinds: []string{"schedule", "installment"}, Lines: []int{4, 5, 6}}}
		if diff := cmp.Diff(want, p.ModuleRecords); diff != "" {
			t.Fatalf("module records (-want +got):\n%s", diff)
		}
		if !strings.Contains(warnings(p), "enable it or the record is skipped") {
			t.Fatalf("warnings = %q", warnings(p))
		}
		for _, c := range p.Changes {
			if c.Entity == "loan" {
				t.Fatalf("module-tagged rows must not be counted: %+v", c)
			}
		}
	})

	t.Run("installment without schedule", func(t *testing.T) {
		e := newEnv(nil)
		src := stream("fui", "", loanAccount("l1", "Home"), instRec("l1", "2026-02-01", 1, 0, 0))
		// The validator rejects this stream structurally: nothing to plan.
		if _, err := e.svc.PlanImport(context.Background(), importing.PlanRequest{Source: src}); err == nil {
			t.Fatal("want structural error")
		}
	})
}
