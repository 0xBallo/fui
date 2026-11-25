package lending

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

// ImportedVersion is a bank-stated schedule version of an existing loan, valid from ValidFrom.
// RateType, when set, replaces the loan's rate type.
type ImportedVersion struct {
	LoanID       domain.LoanID
	ValidFrom    domain.Date
	Rate         domain.AnnualRate
	RateType     domain.RateType
	Installments []domain.Installment
}

// ImportOutcome tells whether ImportVersion wrote anything.
type ImportOutcome string

// Import outcomes.
const (
	ImportCreated   ImportOutcome = "created"
	ImportUnchanged ImportOutcome = "unchanged"
)

// LoanOfAccount returns the loan whose liability account is liability; the bool is false when
// the account belongs to no loan.
func LoanOfAccount(ctx context.Context, r app.Repos, liability domain.AccountID) (domain.Loan, bool, error) {
	loans, err := r.Loans().List(ctx)
	if err != nil {
		return domain.Loan{}, false, fmt.Errorf("list loans: %w", err)
	}
	for _, l := range loans {
		if l.LiabilityAccountID == liability {
			return l, true, nil
		}
	}
	return domain.Loan{}, false, nil
}

// ImportVersion stores v as a new schedule version (origin imported) of its loan, with the
// bank's figures as given. The installments must be in ascending due order starting on or after
// ValidFrom, their principal must add up to the capital outstanding at ValidFrom and the last
// residual must be zero. A version already stored at ValidFrom with the same rate and
// installments is ImportUnchanged; the same date with other content is an already-exists error.
// A loan event at ValidFrom is recorded (except at disbursement) because version dates are
// derived from events. It runs on r, so callers can include it in their own transaction.
func ImportVersion(ctx context.Context, r app.Repos, v ImportedVersion) (ImportOutcome, error) {
	loan, err := r.Loans().Get(ctx, v.LoanID)
	if err != nil {
		return "", fmt.Errorf("get loan: %w", err)
	}
	insts, err := validateImport(loan, v)
	if err != nil {
		return "", err
	}

	prev, hasPrev, err := scheduleAt(ctx, r, loan, v.ValidFrom)
	if err != nil {
		return "", err
	}
	if hasPrev && prev.ValidFrom == v.ValidFrom {
		if prev.Rate == v.Rate && slices.Equal(prev.Installments, insts) {
			return ImportUnchanged, nil
		}
		return "", &domain.Error{
			Code:    app.CodeAlreadyExists,
			Message: fmt.Sprintf("a different schedule version valid from %s already exists", v.ValidFrom),
		}
	}

	outstanding := loan.Principal.Minor()
	if hasPrev {
		outstanding = outstandingAt(prev, v.ValidFrom)
	}
	if err := checkRepayment(insts, v.ValidFrom, outstanding); err != nil {
		return "", err
	}

	return storeVersion(ctx, r, loan, v, insts, prev, hasPrev)
}

// validateImport checks the fields of v that need no stored schedule and returns its
// installments with empty amounts set to zero.
func validateImport(loan domain.Loan, v ImportedVersion) ([]domain.Installment, error) {
	if v.RateType != "" && !v.RateType.Valid() {
		return nil, invalid("rate_type", fmt.Sprintf("unknown rate type %q", v.RateType))
	}
	if v.ValidFrom.IsZero() || v.ValidFrom.Before(loan.DisbursedOn) {
		return nil, invalid("valid_from", "schedule version must start on or after the disbursement")
	}
	if _, err := domain.NewAnnualRate(v.Rate.BasisPoints()); err != nil {
		return nil, err
	}
	return normalizeInstallments(v.Installments, loan.Principal.Currency())
}

func storeVersion(
	ctx context.Context, r app.Repos, loan domain.Loan, v ImportedVersion,
	insts []domain.Installment, prev domain.Schedule, hasPrev bool,
) (ImportOutcome, error) {
	sched := domain.Schedule{
		LoanID: loan.ID, ValidFrom: v.ValidFrom, Origin: domain.ScheduleOriginImported,
		Rate: v.Rate, Installments: insts,
	}
	if err := r.Loans().AddScheduleVersion(ctx, sched); err != nil {
		return "", fmt.Errorf("store schedule: %w", err)
	}
	if v.RateType != "" && v.RateType != loan.RateType {
		loan.RateType = v.RateType
		if err := r.Loans().Update(ctx, loan); err != nil {
			return "", fmt.Errorf("update loan rate type: %w", err)
		}
	}
	if v.ValidFrom != loan.DisbursedOn {
		kind := domain.EventKindRenegotiation
		if hasPrev && prev.Rate != v.Rate {
			kind = domain.EventKindRateChange
		}
		rate := v.Rate
		ev := domain.LoanEvent{LoanID: loan.ID, Kind: kind, EventDate: v.ValidFrom, NewRate: &rate}
		if _, err := r.Loans().InsertEvent(ctx, ev); err != nil {
			return "", fmt.Errorf("store event: %w", err)
		}
	}
	return ImportCreated, nil
}

// scheduleAt returns the version in force at at; the bool is false when there is none.
func scheduleAt(ctx context.Context, r app.Repos, loan domain.Loan, at domain.Date) (domain.Schedule, bool, error) {
	s, err := r.Loans().ScheduleAt(ctx, loan.ID, at)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Schedule{}, false, nil
	}
	if err != nil {
		return domain.Schedule{}, false, fmt.Errorf("schedule at %s: %w", at, err)
	}
	return s, true, nil
}

// normalizeInstallments fills empty amounts with zero in cur and checks the due order.
func normalizeInstallments(in []domain.Installment, cur domain.Currency) ([]domain.Installment, error) {
	if len(in) == 0 {
		return nil, invalid("installments", "imported version has no installments")
	}
	zero, err := domain.NewMoney(0, cur)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Installment, len(in))
	for i, inst := range in {
		for _, m := range []*domain.Money{&inst.Principal, &inst.Interest, &inst.Fees, &inst.Remaining} {
			if m.Currency().IsZero() {
				*m = zero
			}
		}
		if i > 0 && !inst.Due.After(out[i-1].Due) {
			return nil, invalid("installments", "imported installments must be in ascending due order")
		}
		out[i] = inst
	}
	return out, nil
}

// outstandingAt is the capital owed at at under prev: the residual after its last installment
// due before at, or the whole principal of prev when none is.
func outstandingAt(prev domain.Schedule, at domain.Date) int64 {
	var total int64
	for _, inst := range prev.Installments {
		total += inst.Principal.Minor()
	}
	for i := len(prev.Installments) - 1; i >= 0; i-- {
		if prev.Installments[i].Due.Before(at) {
			return prev.Installments[i].Remaining.Minor()
		}
	}
	return total
}

func checkRepayment(insts []domain.Installment, from domain.Date, outstanding int64) error {
	if insts[0].Due.Before(from) {
		return invalid("installments", "first installment is due before the version start")
	}
	var sum int64
	for _, inst := range insts {
		sum += inst.Principal.Minor()
	}
	if sum != outstanding {
		return invalid("installments", fmt.Sprintf("principal sums to %d, outstanding capital is %d", sum, outstanding))
	}
	if insts[len(insts)-1].Remaining.Minor() != 0 {
		return invalid("installments", "final residual must be zero")
	}
	return nil
}
