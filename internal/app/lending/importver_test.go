package lending_test

import (
	"context"
	"errors"
	"testing"

	"pgregory.net/rapid"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/lending"
	"github.com/0xballo/fui/internal/domain"
)

func importVersion(t tb, e env, v lending.ImportedVersion) (lending.ImportOutcome, error) {
	t.Helper()
	var out lending.ImportOutcome
	err := e.store.WithTx(context.Background(), func(r app.Repos) (err error) {
		out, err = lending.ImportVersion(context.Background(), r, v)
		return err
	})
	return out, err
}

// tail builds installments of 100 principal each from the given dues; the last residual is zero
// and the first one leaves outstanding minus 100.
func tail(outstanding int64, dues ...string) []domain.Installment {
	n := int64(len(dues))
	step := outstanding / n
	out := make([]domain.Installment, len(dues))
	left := outstanding
	for i, d := range dues {
		p := step
		if int64(i) == n-1 {
			p = left
		}
		left -= p
		out[i] = domain.Installment{Due: date(d), Principal: money(p), Interest: money(7), Remaining: money(left)}
	}
	return out
}

func outstandingAfter(t tb, e env, loan domain.Loan, at domain.Date) int64 {
	t.Helper()
	var s domain.Schedule
	err := e.store.Read(context.Background(), func(r app.Repos) (err error) {
		s, err = r.Loans().ScheduleAt(context.Background(), loan.ID, at)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	out := loan.Principal.Minor()
	for _, inst := range s.Installments {
		if inst.Due.Before(at) {
			out = inst.Remaining.Minor()
		}
	}
	return out
}

func TestImportVersion(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	// Imported-from-scratch loan: a bare loan row without schedule.
	var loan domain.Loan
	err := e.store.WithTx(ctx, func(r app.Repos) error {
		acct, err := r.Accounts().Insert(ctx, domain.Account{
			Name: "Car", Kind: domain.AccountLoan, Currency: eur(), OpeningBalance: money(-1000), OpenedOn: date("2026-01-01"),
		})
		if err != nil {
			return err
		}
		loan, err = r.Loans().Insert(ctx, domain.Loan{
			Name: "Car", LiabilityAccountID: acct.ID, Kind: domain.LoanKindLoan, Principal: money(1000),
			DisbursedOn: date("2026-01-01"), TermMonths: 4, RateType: domain.RateTypeFixed, DebitAccount: e.main.ID,
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	first := lending.ImportedVersion{
		LoanID: loan.ID, ValidFrom: date("2026-01-01"), Rate: 300, RateType: domain.RateTypeVariable,
		Installments: tail(1000, "2026-02-01", "2026-03-01", "2026-04-01", "2026-05-01"),
	}

	t.Run("found by account", func(t *testing.T) {
		_ = e.store.Read(ctx, func(r app.Repos) error {
			got, ok, err := lending.LoanOfAccount(ctx, r, loan.LiabilityAccountID)
			if err != nil || !ok || got.ID != loan.ID {
				t.Errorf("LoanOfAccount = %v %v %v", got.ID, ok, err)
			}
			if _, ok, _ := lending.LoanOfAccount(ctx, r, e.main.ID); ok {
				t.Error("main account matched a loan")
			}
			return nil
		})
	})

	for _, bad := range []struct {
		name string
		mut  func(v *lending.ImportedVersion)
	}{
		{"bad sum", func(v *lending.ImportedVersion) { v.Installments[0].Principal = money(1) }},
		{"bad residual", func(v *lending.ImportedVersion) { v.Installments[3].Remaining = money(5) }},
		{"unordered", func(v *lending.ImportedVersion) { v.Installments[1].Due = date("2026-01-15") }},
		{"first due before start", func(v *lending.ImportedVersion) { v.ValidFrom = date("2026-03-01") }},
		{"before disbursement", func(v *lending.ImportedVersion) { v.ValidFrom = date("2025-12-01") }},
		{"empty", func(v *lending.ImportedVersion) { v.Installments = nil }},
		{"rate type", func(v *lending.ImportedVersion) { v.RateType = "bogus" }},
	} {
		t.Run(bad.name, func(t *testing.T) {
			v := first
			v.Installments = append([]domain.Installment(nil), first.Installments...)
			bad.mut(&v)
			_, err := importVersion(t, e, v)
			var de *domain.Error
			if !errors.As(err, &de) {
				t.Fatalf("err = %v, want *domain.Error", err)
			}
		})
	}

	t.Run("first version", func(t *testing.T) {
		out, err := importVersion(t, e, first)
		if err != nil || out != lending.ImportCreated {
			t.Fatalf("out=%v err=%v", out, err)
		}
		_ = e.store.Read(ctx, func(r app.Repos) error {
			l, _ := r.Loans().Get(ctx, loan.ID)
			if l.RateType != domain.RateTypeVariable {
				t.Errorf("rate type = %v", l.RateType)
			}
			if ev, _ := r.Loans().ListEvents(ctx, loan.ID); len(ev) != 0 {
				t.Errorf("events at disbursement: %v", ev)
			}
			s, _ := r.Loans().ScheduleAt(ctx, loan.ID, date("2026-01-01"))
			if s.Origin != domain.ScheduleOriginImported || s.Installments[0].Interest.Minor() != 7 {
				t.Errorf("schedule = %+v", s)
			}
			return nil
		})
	})

	t.Run("duplicate same", func(t *testing.T) {
		out, err := importVersion(t, e, first)
		if err != nil || out != lending.ImportUnchanged {
			t.Errorf("out=%v err=%v", out, err)
		}
	})

	t.Run("duplicate different", func(t *testing.T) {
		v := first
		v.Rate = 400
		if _, err := importVersion(t, e, v); !errors.Is(err, app.ErrAlreadyExists) {
			t.Errorf("err = %v", err)
		}
	})

	t.Run("renegotiation with event", func(t *testing.T) {
		// 2 installments paid before 2026-03-15 leaves 500.
		v := lending.ImportedVersion{
			LoanID: loan.ID, ValidFrom: date("2026-03-15"), Rate: 300,
			Installments: []domain.Installment{
				{Due: date("2026-04-01"), Principal: money(200), Remaining: money(300)},
				{Due: date("2026-05-01"), Principal: money(300), Fees: money(2)},
			},
		}
		if got := outstandingAfter(t, e, loan, v.ValidFrom); got != 500 {
			t.Fatalf("outstanding = %d", got)
		}
		out, err := importVersion(t, e, v)
		if err != nil || out != lending.ImportCreated {
			t.Fatalf("out=%v err=%v", out, err)
		}
		_ = e.store.Read(ctx, func(r app.Repos) error {
			ev, _ := r.Loans().ListEvents(ctx, loan.ID)
			if len(ev) != 1 || ev[0].Kind != domain.EventKindRenegotiation || ev[0].EventDate != v.ValidFrom ||
				ev[0].NewRate == nil || *ev[0].NewRate != 300 {
				t.Errorf("events = %+v", ev)
			}
			old, _ := r.Loans().ScheduleAt(ctx, loan.ID, date("2026-03-14"))
			if len(old.Installments) != 4 || old.ValidFrom != date("2026-01-01") {
				t.Errorf("older version touched: %+v", old)
			}
			cur, _ := r.Loans().ScheduleAt(ctx, loan.ID, v.ValidFrom)
			if cur.Installments[0].Interest.Currency().IsZero() {
				t.Error("empty money not defaulted")
			}
			return nil
		})
		if out, err = importVersion(t, e, v); err != nil || out != lending.ImportUnchanged {
			t.Errorf("re-import out=%v err=%v", out, err)
		}
	})

	t.Run("rate change kind", func(t *testing.T) {
		v := lending.ImportedVersion{
			LoanID: loan.ID, ValidFrom: date("2026-04-15"), Rate: 450,
			Installments: []domain.Installment{{Due: date("2026-05-01"), Principal: money(300)}},
		}
		if _, err := importVersion(t, e, v); err != nil {
			t.Fatal(err)
		}
		_ = e.store.Read(ctx, func(r app.Repos) error {
			ev, _ := r.Loans().ListEvents(ctx, loan.ID)
			if len(ev) != 2 || ev[1].Kind != domain.EventKindRateChange {
				t.Errorf("events = %+v", ev)
			}
			return nil
		})
	})

	t.Run("pending installments follow versions", func(t *testing.T) {
		if n := e.generate(t, "2026-12-31"); n == 0 {
			t.Error("no pending installments generated")
		}
	})

	t.Run("unknown loan", func(t *testing.T) {
		v := first
		v.LoanID = 999
		if _, err := importVersion(t, e, v); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("err = %v", err)
		}
	})
}

func TestImportVersionRoundTripProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		e := newEnv(t)
		ctx := context.Background()
		req := addReq()
		req.Principal = rapid.StringMatching(`[1-9][0-9]{3,5}`).Draw(t, "principal")
		req.TermMonths = rapid.IntRange(3, 60).Draw(t, "term")
		req.Rate = rapid.SampledFrom([]string{"0,5", "2,75", "4", "9,99"}).Draw(t, "rate")
		info := e.add(t, req)

		var base domain.Schedule
		if err := e.store.Read(ctx, func(r app.Repos) (err error) {
			base, err = r.Loans().ScheduleAt(ctx, info.Loan.ID, info.Loan.DisbursedOn)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		k := rapid.IntRange(1, len(base.Installments)-1).Draw(t, "boundary")
		from := base.Installments[k-1].Due
		// Start the version the day after the k-th installment: k installments are in the past.
		validFrom, err := from.AddDays(1)
		if err != nil {
			t.Fatal(err)
		}
		want := base.Installments[k-1].Remaining.Minor()

		tailInsts := append([]domain.Installment(nil), base.Installments[k:]...)
		v := lending.ImportedVersion{LoanID: info.Loan.ID, ValidFrom: validFrom, Rate: base.Rate, Installments: tailInsts}
		out, err := importVersion(t, e, v)
		if err != nil || out != lending.ImportCreated {
			t.Fatalf("out=%v err=%v", out, err)
		}
		var sum int64
		for _, inst := range tailInsts {
			sum += inst.Principal.Minor()
		}
		if sum != want || tailInsts[len(tailInsts)-1].Remaining.Minor() != 0 {
			t.Fatalf("sum=%d want=%d", sum, want)
		}
		if out, err = importVersion(t, e, v); err != nil || out != lending.ImportUnchanged {
			t.Fatalf("re-import out=%v err=%v", out, err)
		}
	})
}
