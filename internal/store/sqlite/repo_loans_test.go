package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

func TestLoanRepoCRUDScheduleEventsAndLink(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	eur := mustCurrency(t, "EUR")
	principal := mustMoney(t, 100_000_00, eur)
	rate, err := domain.NewAnnualRate(350)
	if err != nil {
		t.Fatal(err)
	}

	var debit, loanAcc domain.Account
	if err := store.WithTx(ctx, func(r app.Repos) error {
		var txErr error
		debit, txErr = r.Accounts().Insert(ctx, domain.Account{
			Name: "Loan debit", Kind: domain.AccountChecking, Currency: eur,
			OpeningBalance: mustMoney(t, 0, eur), OpenedOn: mustDate(t, "2026-01-01"),
		})
		if txErr != nil {
			return txErr
		}
		loanAcc, txErr = r.Accounts().Insert(ctx, domain.Account{
			Name: "Loan account", Kind: domain.AccountLoan, Currency: eur,
			OpeningBalance: mustMoney(t, -1000, eur), OpenedOn: mustDate(t, "2026-01-01"),
		})
		return txErr
	}); err != nil {
		t.Fatalf("seed accounts: %v", err)
	}

	var loan domain.Loan
	var installmentTx domain.Transaction
	schedule := domain.Schedule{
		ValidFrom: mustDate(t, "2026-02-01"),
		Origin:    domain.ScheduleOriginComputed,
		Rate:      rate,
		Installments: []domain.Installment{
			{
				Due:       mustDate(t, "2026-03-01"),
				Principal: mustMoney(t, 4_000_00, eur),
				Interest:  mustMoney(t, 1_000_00, eur),
				Fees:      mustMoney(t, 50_00, eur),
				Remaining: mustMoney(t, 96_000_00, eur),
			},
		},
	}

	if err := store.WithTx(ctx, func(r app.Repos) error {
		var txErr error
		loan, txErr = r.Loans().Insert(ctx, domain.Loan{
			Kind:         domain.LoanKindMortgage,
			Principal:    principal,
			DisbursedOn:  mustDate(t, "2026-01-01"),
			TermMonths:   24,
			RateType:     domain.RateTypeFixed,
			DebitAccount: debit.ID,
		})
		if txErr != nil {
			return txErr
		}
		schedule.LoanID = loan.ID
		if txErr = r.Loans().AddScheduleVersion(ctx, schedule); txErr != nil {
			return txErr
		}
		got, txErr := r.Loans().ScheduleAt(ctx, loan.ID, mustDate(t, "2026-03-15"))
		if txErr != nil {
			return txErr
		}
		if len(got.Installments) != 1 || got.Installments[0].Due != schedule.Installments[0].Due {
			t.Fatalf("ScheduleAt installments = %+v", got.Installments)
		}
		installmentTx, txErr = r.Transactions().Insert(ctx, domain.Transaction{
			AccountID:   loanAcc.ID,
			Date:        mustDate(t, "2026-03-01"),
			Amount:      mustMoney(t, -505_00, eur),
			Description: "installment",
			Status:      domain.StatusBooked,
			Kind:        domain.KindPayment,
		})
		if txErr != nil {
			return txErr
		}
		if txErr = r.Loans().LinkInstallment(ctx, loan.ID, schedule.ValidFrom, schedule.Installments[0].Due, installmentTx.ID); txErr != nil {
			return txErr
		}
		if txErr = r.Loans().UnlinkInstallment(ctx, loan.ID, schedule.ValidFrom, schedule.Installments[0].Due); txErr != nil {
			return txErr
		}
		mode := domain.PrepaymentReduceInstallment
		amount := mustMoney(t, 1_000_00, eur)
		_, txErr = r.Loans().InsertEvent(ctx, domain.LoanEvent{
			LoanID:        loan.ID,
			Kind:          domain.EventKindPrepayment,
			EventDate:     mustDate(t, "2026-05-01"),
			PrepaidAmount: &amount,
			PrepayMode:    &mode,
		})
		if txErr != nil {
			return txErr
		}
		events, txErr := r.Loans().ListEvents(ctx, loan.ID)
		if txErr != nil {
			return txErr
		}
		if len(events) != 1 || events[0].Kind != domain.EventKindPrepayment {
			t.Fatalf("ListEvents = %+v", events)
		}
		loan.TermMonths = 36
		if txErr = r.Loans().Update(ctx, loan); txErr != nil {
			return txErr
		}
		list, txErr := r.Loans().List(ctx)
		if txErr != nil {
			return txErr
		}
		if len(list) != 1 || list[0].ID != loan.ID {
			t.Fatalf("List = %+v", list)
		}
		return nil
	}); err != nil {
		t.Fatalf("loan flow: %v", err)
	}

	if err := store.WithTx(ctx, func(r app.Repos) error {
		return r.Loans().Delete(ctx, loan.ID)
	}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := store.Read(ctx, func(r app.Repos) error {
		_, err := r.Loans().Get(ctx, loan.ID)
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("Get after delete err = %v, want not_found", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestLoanRepoValidationAndNotFound(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	eur := mustCurrency(t, "EUR")
	principal := mustMoney(t, 100_000_00, eur)
	rate, _ := domain.NewAnnualRate(300)

	if err := store.WithTx(ctx, func(r app.Repos) error {
		_, err := r.Loans().Insert(ctx, domain.Loan{
			Kind: domain.LoanKindLoan, Principal: principal, DisbursedOn: mustDate(t, "2026-01-01"),
			TermMonths: 12, RateType: domain.RateTypeFixed, DebitAccount: 999,
		})
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("Insert missing account err = %v, want not_found", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	var account domain.Account
	if err := store.WithTx(ctx, func(r app.Repos) error {
		var txErr error
		account, txErr = r.Accounts().Insert(ctx, domain.Account{
			Name: "A", Kind: domain.AccountChecking, Currency: eur,
			OpeningBalance: mustMoney(t, 0, eur), OpenedOn: mustDate(t, "2026-01-01"),
		})
		return txErr
	}); err != nil {
		t.Fatal(err)
	}

	if err := store.WithTx(ctx, func(r app.Repos) error {
		loan, txErr := r.Loans().Insert(ctx, domain.Loan{
			Kind: domain.LoanKindLoan, Principal: principal, DisbursedOn: mustDate(t, "2026-01-01"),
			TermMonths: 12, RateType: domain.RateTypeFixed, DebitAccount: account.ID,
		})
		if txErr != nil {
			return txErr
		}
		usd := mustCurrency(t, "USD")
		bad := domain.Schedule{
			LoanID: loan.ID, ValidFrom: mustDate(t, "2026-02-01"), Origin: domain.ScheduleOriginComputed, Rate: rate,
			Installments: []domain.Installment{
				{
					Due:       mustDate(t, "2026-03-01"),
					Principal: mustMoney(t, 1_000_00, usd),
					Interest:  mustMoney(t, 100_00, usd),
					Fees:      mustMoney(t, 0, usd),
					Remaining: mustMoney(t, 99_000_00, usd),
				},
			},
		}
		if txErr = r.Loans().AddScheduleVersion(ctx, bad); !errors.Is(txErr, domain.ErrCurrencyMismatch) {
			t.Fatalf("AddScheduleVersion bad currency err = %v, want currency_mismatch", txErr)
		}
		if txErr = r.Loans().LinkInstallment(ctx, loan.ID, mustDate(t, "2026-02-01"), mustDate(t, "2026-03-01"), 999); !errors.Is(txErr, domain.ErrNotFound) {
			t.Fatalf("LinkInstallment missing tx err = %v, want not_found", txErr)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
