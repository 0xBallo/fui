package memstore

import (
	"context"
	"errors"
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

func contractLoans(t *testing.T, f *fixture) {
	debit := f.account(t, "Loan debit")
	loanAcc := f.account(t, "Loan account")
	loan := insertContractLoan(t, f, debit.ID)
	schedule := makeContractSchedule(t, loan.ID)
	linkContractInstallment(t, f, loan.ID, schedule, loanAcc.ID)
	contractLoanEvents(t, f, loan.ID)
	contractLoanUpdateDelete(t, f, loan, loanAcc.ID)
}

func insertContractLoan(t *testing.T, f *fixture, debitAccountID domain.AccountID) domain.Loan {
	t.Helper()
	ctx := context.Background()
	principal, err := domain.NewMoney(100_000_00, mustEUR(t))
	if err != nil {
		t.Fatal(err)
	}
	var loan domain.Loan
	f.do(t, func(r app.Repos) error {
		loan, err = r.Loans().Insert(ctx, domain.Loan{
			Kind:         domain.LoanKindMortgage,
			Principal:    principal,
			DisbursedOn:  mustDate(t, "2026-01-01"),
			TermMonths:   24,
			RateType:     domain.RateTypeFixed,
			DebitAccount: debitAccountID,
		})
		if err != nil {
			return err
		}
		if _, err := r.Loans().Get(ctx, loan.ID); err != nil {
			t.Errorf("Get inserted loan: %v", err)
		}
		_, err = r.Loans().Insert(ctx, domain.Loan{
			Kind:         domain.LoanKindLoan,
			Principal:    principal,
			DisbursedOn:  mustDate(t, "2026-01-01"),
			TermMonths:   12,
			RateType:     domain.RateTypeFixed,
			DebitAccount: 999,
		})
		if !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("Insert missing debit account err = %v, want not_found", err)
		}
		return nil
	})
	if loan.ID == 0 {
		t.Fatal("loan id not assigned")
	}
	return loan
}

func makeContractSchedule(t *testing.T, loanID domain.LoanID) domain.Schedule {
	t.Helper()
	rate, err := domain.NewAnnualRate(350)
	if err != nil {
		t.Fatal(err)
	}
	return domain.Schedule{
		LoanID:    loanID,
		ValidFrom: mustDate(t, "2026-02-01"),
		Origin:    domain.ScheduleOriginComputed,
		Rate:      rate,
		Installments: []domain.Installment{
			{
				Due:       mustDate(t, "2026-03-01"),
				Principal: mustMoney(t, 4_000_00),
				Interest:  mustMoney(t, 1_000_00),
				Fees:      mustMoney(t, 50_00),
				Remaining: mustMoney(t, 96_000_00),
			},
			{
				Due:       mustDate(t, "2026-04-01"),
				Principal: mustMoney(t, 4_200_00),
				Interest:  mustMoney(t, 800_00),
				Fees:      mustMoney(t, 50_00),
				Remaining: mustMoney(t, 91_800_00),
			},
		},
	}
}

func linkContractInstallment(
	t *testing.T, f *fixture, loanID domain.LoanID, schedule domain.Schedule, txAccount domain.AccountID,
) {
	t.Helper()
	ctx := context.Background()
	f.do(t, func(r app.Repos) error {
		if err := r.Loans().AddScheduleVersion(ctx, schedule); err != nil {
			return err
		}
		if err := r.Loans().AddScheduleVersion(ctx, schedule); !errors.Is(err, app.ErrAlreadyExists) {
			t.Errorf("duplicate AddScheduleVersion err = %v, want already_exists", err)
		}
		got, err := r.Loans().ScheduleAt(ctx, loanID, mustDate(t, "2026-03-15"))
		if err != nil {
			t.Errorf("ScheduleAt: %v", err)
		}
		if got.ValidFrom != schedule.ValidFrom || len(got.Installments) != 2 || got.Installments[0].Due != schedule.Installments[0].Due {
			t.Errorf("ScheduleAt = %+v, want %+v", got, schedule)
		}
		if _, err := r.Loans().ScheduleAt(ctx, loanID, mustDate(t, "2026-01-15")); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("ScheduleAt before first err = %v, want not_found", err)
		}
		tx, err := r.Transactions().Insert(ctx, domain.Transaction{
			AccountID:   txAccount,
			Date:        mustDate(t, "2026-03-01"),
			Amount:      mustMoney(t, -505_00),
			Description: "installment",
			Status:      domain.StatusBooked,
			Kind:        domain.KindPayment,
		})
		if err != nil {
			return err
		}
		if err := r.Loans().LinkInstallment(ctx, loanID, schedule.ValidFrom, schedule.Installments[0].Due, tx.ID); err != nil {
			t.Errorf("LinkInstallment: %v", err)
		}
		if err := r.Loans().UnlinkInstallment(ctx, loanID, schedule.ValidFrom, schedule.Installments[0].Due); err != nil {
			t.Errorf("UnlinkInstallment: %v", err)
		}
		if err := r.Loans().LinkInstallment(ctx, loanID, schedule.ValidFrom, schedule.Installments[0].Due, 999); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("LinkInstallment missing tx err = %v, want not_found", err)
		}
		return nil
	})
}

func contractLoanEvents(t *testing.T, f *fixture, loanID domain.LoanID) {
	t.Helper()
	ctx := context.Background()
	prepayMode := domain.PrepaymentReduceTerm
	prepayAmount := mustMoney(t, 1_000_00)
	newTerm := 20
	f.do(t, func(r app.Repos) error {
		_, err := r.Loans().InsertEvent(ctx, domain.LoanEvent{
			LoanID:        loanID,
			Kind:          domain.EventKindPrepayment,
			EventDate:     mustDate(t, "2026-05-01"),
			NewTermMonths: &newTerm,
			PrepaidAmount: &prepayAmount,
			PrepayMode:    &prepayMode,
		})
		if err != nil {
			return err
		}
		_, err = r.Loans().InsertEvent(ctx, domain.LoanEvent{
			LoanID:    loanID,
			Kind:      domain.EventKindRateChange,
			EventDate: mustDate(t, "2026-04-15"),
		})
		if err != nil {
			return err
		}
		events, err := r.Loans().ListEvents(ctx, loanID)
		if err != nil {
			return err
		}
		if len(events) != 2 || events[0].EventDate.After(events[1].EventDate) {
			t.Errorf("ListEvents order = %+v", events)
		}
		return nil
	})
}

func contractLoanUpdateDelete(t *testing.T, f *fixture, loan domain.Loan, debit domain.AccountID) {
	t.Helper()
	ctx := context.Background()
	f.do(t, func(r app.Repos) error {
		loan.DebitAccount = debit
		if err := r.Loans().Update(ctx, loan); err != nil {
			t.Errorf("Update: %v", err)
		}
		all, err := r.Loans().List(ctx)
		if err != nil || len(all) != 1 {
			t.Errorf("List = %+v, %v", all, err)
		}
		if err := r.Loans().Delete(ctx, loan.ID); err != nil {
			t.Errorf("Delete: %v", err)
		}
		if _, err := r.Loans().Get(ctx, loan.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("Get after delete err = %v, want not_found", err)
		}
		return nil
	})
}
