package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	gen "github.com/0xballo/fui/internal/store/sqlite/sqlc"
)

func TestLoanConvertersAndValidators(t *testing.T) {
	eur := mustCurrency(t, "EUR")
	loan, err := toLoan(gen.Loan{
		ID: 1, Name: "L", LiabilityAccountID: sql.NullInt64{Int64: 3, Valid: true}, Kind: "loan", PrincipalMinor: 100_000_00, Currency: "EUR",
		DisbursedOn: "2026-01-01", TermMonths: 24, RateType: "fixed", DebitAccountID: 2,
	})
	if err != nil {
		t.Fatalf("toLoan(valid): %v", err)
	}
	if loan.ID != 1 || loan.DebitAccount != 2 {
		t.Fatalf("toLoan(valid) = %+v", loan)
	}
	if _, err := toLoan(gen.Loan{Name: "L", LiabilityAccountID: sql.NullInt64{Int64: 3, Valid: true}, Kind: "loan", PrincipalMinor: 1, Currency: "XXX", DisbursedOn: "2026-01-01", TermMonths: 1, RateType: "fixed", DebitAccountID: 1}); err == nil {
		t.Fatal("toLoan accepted invalid currency")
	}

	rate, _ := domain.NewAnnualRate(350)
	installments := []gen.LoanInstallment{
		{DueDate: "2026-03-01", PrincipalMinor: 4_000_00, InterestMinor: 1_000_00, FeesMinor: 50_00, RemainingMinor: 96_000_00},
	}
	schedule, err := toSchedule(gen.LoanSchedule{
		ID: 1, LoanID: 1, ValidFrom: "2026-02-01", Origin: "computed", RateBasisPoints: 350,
	}, installments, eur)
	if err != nil {
		t.Fatalf("toSchedule(valid): %v", err)
	}
	if schedule.Rate != rate || len(schedule.Installments) != 1 {
		t.Fatalf("toSchedule(valid) = %+v", schedule)
	}
	if _, err := toSchedule(gen.LoanSchedule{LoanID: 1, ValidFrom: "2026-02-01", Origin: "bad", RateBasisPoints: 300}, nil, eur); err == nil {
		t.Fatal("toSchedule accepted invalid origin")
	}
	if _, err := toSchedule(gen.LoanSchedule{LoanID: 1, ValidFrom: "2026-02-01", Origin: "computed", RateBasisPoints: 300},
		[]gen.LoanInstallment{{DueDate: "bad-date"}}, eur); err == nil {
		t.Fatal("toSchedule accepted invalid due date")
	}

	loan = domain.Loan{ID: 1, Principal: mustMoney(t, 100_000_00, eur)}
	ok := domain.Schedule{
		LoanID:    1,
		ValidFrom: mustDate(t, "2026-02-01"),
		Origin:    domain.ScheduleOriginComputed,
		Rate:      rate,
		Installments: []domain.Installment{
			{
				Due:       mustDate(t, "2026-03-01"),
				Principal: mustMoney(t, 1, eur),
				Interest:  mustMoney(t, 1, eur),
				Fees:      mustMoney(t, 0, eur),
				Remaining: mustMoney(t, 1, eur),
			},
		},
	}
	if err := validateScheduleVersion(ok, loan); err != nil {
		t.Fatalf("validateScheduleVersion(valid): %v", err)
	}
	bad := ok
	bad.LoanID = 2
	if err := validateScheduleVersion(bad, loan); !errors.Is(err, domain.ErrInvalidValue) {
		t.Fatalf("validateScheduleVersion loan mismatch err = %v", err)
	}
	bad = ok
	bad.Origin = "bad"
	if err := validateScheduleVersion(bad, loan); !errors.Is(err, domain.ErrInvalidValue) {
		t.Fatalf("validateScheduleVersion invalid origin err = %v", err)
	}
}

func TestLoanEventConverters(t *testing.T) {
	eur := mustCurrency(t, "EUR")
	newRate, _ := domain.NewAnnualRate(300)
	newTerm := 12
	mode := domain.PrepaymentReduceTerm
	prepaid := mustMoney(t, 100_00, eur)
	valid := domain.LoanEvent{
		LoanID: 1, Kind: domain.EventKindPrepayment, EventDate: mustDate(t, "2026-04-01"),
		NewRate: &newRate, NewTermMonths: &newTerm, PrepaidAmount: &prepaid, PrepayMode: &mode,
	}
	if _, err := toInsertLoanEventParams(valid, eur); err != nil {
		t.Fatalf("toInsertLoanEventParams(valid): %v", err)
	}
	invalidKind := valid
	invalidKind.Kind = "bad"
	if _, err := toInsertLoanEventParams(invalidKind, eur); !errors.Is(err, domain.ErrInvalidValue) {
		t.Fatalf("toInsertLoanEventParams invalid kind err = %v", err)
	}
	missingDate := valid
	missingDate.EventDate = domain.Date{}
	if _, err := toInsertLoanEventParams(missingDate, eur); !errors.Is(err, domain.ErrMissingField) {
		t.Fatalf("toInsertLoanEventParams missing date err = %v", err)
	}
	usd := mustCurrency(t, "USD")
	badCurrencyAmount := mustMoney(t, 100_00, usd)
	invalidCurrency := valid
	invalidCurrency.PrepaidAmount = &badCurrencyAmount
	if _, err := toInsertLoanEventParams(invalidCurrency, eur); !errors.Is(err, domain.ErrCurrencyMismatch) {
		t.Fatalf("toInsertLoanEventParams bad currency err = %v", err)
	}

	event, err := toLoanEvent(gen.LoanEvent{
		ID: 1, LoanID: 1, Kind: "prepayment", EventDate: "2026-04-01",
		NewRateBasisPoints: sql.NullInt64{Int64: 300, Valid: true},
		NewTermMonths:      sql.NullInt64{Int64: 10, Valid: true},
		PrepaidMinor:       sql.NullInt64{Int64: 100, Valid: true},
		PrepayMode:         sql.NullString{String: "reduce_term", Valid: true},
	}, eur)
	if err != nil {
		t.Fatalf("toLoanEvent(valid): %v", err)
	}
	if event.NewRate == nil || event.PrepaidAmount == nil || event.PrepayMode == nil {
		t.Fatalf("toLoanEvent(valid) = %+v", event)
	}
	if _, err := toLoanEvent(gen.LoanEvent{
		LoanID: 1, Kind: "prepayment", EventDate: "2026-04-01", PrepayMode: sql.NullString{String: "bad", Valid: true},
	}, eur); err == nil {
		t.Fatal("toLoanEvent accepted invalid prepay mode")
	}
	if _, err := toLoanEvent(gen.LoanEvent{LoanID: 1, Kind: "bad", EventDate: "2026-04-01"}, eur); err == nil {
		t.Fatal("toLoanEvent accepted invalid kind")
	}
}

func TestLoanRepoErrorBranches(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	eur := mustCurrency(t, "EUR")
	principal := mustMoney(t, 100_000_00, eur)

	if err := store.WithTx(ctx, func(r app.Repos) error {
		err := r.Loans().Update(ctx, domain.Loan{
			ID: 1, Name: "L", LiabilityAccountID: 3, Kind: domain.LoanKindLoan, Principal: principal, DisbursedOn: mustDate(t, "2026-01-01"),
			TermMonths: 12, RateType: domain.RateTypeFixed, DebitAccount: 1,
		})
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("Update missing err = %v, want not_found", err)
		}
		err = r.Loans().Delete(ctx, 1)
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("Delete missing err = %v, want not_found", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	var loan domain.Loan
	var schedule domain.Schedule
	if err := store.WithTx(ctx, func(r app.Repos) error {
		acct, err := r.Accounts().Insert(ctx, domain.Account{
			Name: "Debit", Kind: domain.AccountChecking, Currency: eur,
			OpeningBalance: mustMoney(t, 0, eur), OpenedOn: mustDate(t, "2026-01-01"),
		})
		if err != nil {
			return err
		}
		liab, err := r.Accounts().Insert(ctx, domain.Account{
			Name: "Debt", Kind: domain.AccountLoan, Currency: eur,
			OpeningBalance: mustMoney(t, -1, eur), OpenedOn: mustDate(t, "2026-01-01"),
		})
		if err != nil {
			return err
		}
		loan, err = r.Loans().Insert(ctx, domain.Loan{
			Name: "Debt", LiabilityAccountID: liab.ID, Kind: domain.LoanKindLoan, Principal: principal, DisbursedOn: mustDate(t, "2026-01-01"),
			TermMonths: 12, RateType: domain.RateTypeFixed, DebitAccount: acct.ID,
		})
		if err != nil {
			return err
		}
		rate, _ := domain.NewAnnualRate(300)
		schedule = domain.Schedule{
			LoanID: loan.ID, ValidFrom: mustDate(t, "2026-02-01"), Origin: domain.ScheduleOriginComputed, Rate: rate,
			Installments: []domain.Installment{
				{Due: mustDate(t, "2026-03-01"), Principal: mustMoney(t, 1, eur), Interest: mustMoney(t, 1, eur), Fees: mustMoney(t, 0, eur), Remaining: mustMoney(t, 1, eur)},
			},
		}
		return r.Loans().AddScheduleVersion(ctx, schedule)
	}); err != nil {
		t.Fatal(err)
	}

	if err := store.Read(ctx, func(r app.Repos) error {
		_, err := r.Loans().ScheduleAt(ctx, loan.ID, domain.Date{})
		if !errors.Is(err, domain.ErrMissingField) {
			t.Fatalf("ScheduleAt zero date err = %v, want missing_field", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := store.WithTx(ctx, func(r app.Repos) error {
		err := r.Loans().UnlinkInstallment(ctx, loan.ID, mustDate(t, "2026-01-01"), schedule.Installments[0].Due)
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("Unlink missing schedule err = %v, want not_found", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
