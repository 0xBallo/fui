package memstore

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

type loanScheduleKey struct {
	loanID    domain.LoanID
	validFrom domain.Date
}

type loanInstallmentKey struct {
	loanID       domain.LoanID
	scheduleFrom domain.Date
	due          domain.Date
}

type loans struct{ *repos }

func cloneSchedules(src map[loanScheduleKey]domain.Schedule) map[loanScheduleKey]domain.Schedule {
	dst := make(map[loanScheduleKey]domain.Schedule, len(src))
	for k, v := range src {
		dst[k] = cloneSchedule(v)
	}
	return dst
}

func cloneEvents(src map[domain.LoanID][]domain.LoanEvent) map[domain.LoanID][]domain.LoanEvent {
	dst := make(map[domain.LoanID][]domain.LoanEvent, len(src))
	for k, v := range src {
		cloned := make([]domain.LoanEvent, 0, len(v))
		for _, event := range v {
			cloned = append(cloned, cloneEvent(event))
		}
		dst[k] = cloned
	}
	return dst
}

func cloneSchedule(schedule domain.Schedule) domain.Schedule {
	out := schedule
	out.Installments = slices.Clone(schedule.Installments)
	return out
}

func cloneEvent(event domain.LoanEvent) domain.LoanEvent {
	out := event
	if event.NewRate != nil {
		rate := *event.NewRate
		out.NewRate = &rate
	}
	if event.NewTermMonths != nil {
		term := *event.NewTermMonths
		out.NewTermMonths = &term
	}
	if event.PrepaidAmount != nil {
		amount := *event.PrepaidAmount
		out.PrepaidAmount = &amount
	}
	if event.PrepayMode != nil {
		mode := *event.PrepayMode
		out.PrepayMode = &mode
	}
	return out
}

// checkIdentity enforces what the SQL schema does: the liability account exists and is of kind
// loan, and neither the name (case-insensitively) nor the liability account is used by another loan.
func (l loans) checkIdentity(loan domain.Loan) error {
	if loan.LiabilityAccountID != 0 {
		acct, ok := l.st.Accounts[loan.LiabilityAccountID]
		if !ok {
			return notFound("account", loan.LiabilityAccountID)
		}
		if acct.Kind != domain.AccountLoan {
			return fmt.Errorf("loan liability account %d is of kind %s: %w", acct.ID, acct.Kind, domain.ErrInvalidValue)
		}
	}
	for id, other := range l.st.Loans {
		if id == loan.ID {
			continue
		}
		if strings.EqualFold(other.Name, loan.Name) {
			return exists("loan", loan.Name)
		}
		if loan.LiabilityAccountID != 0 && other.LiabilityAccountID == loan.LiabilityAccountID {
			return exists("loan liability account", fmt.Sprint(loan.LiabilityAccountID))
		}
	}
	return nil
}

func (l loans) Insert(_ context.Context, loan domain.Loan) (domain.Loan, error) {
	if err := l.writable(); err != nil {
		return domain.Loan{}, err
	}
	norm, err := domain.NewLoan(loan)
	if err != nil {
		return domain.Loan{}, err
	}
	if _, ok := l.st.Accounts[norm.DebitAccount]; !ok {
		return domain.Loan{}, notFound("account", norm.DebitAccount)
	}
	if err := l.checkIdentity(norm); err != nil {
		return domain.Loan{}, err
	}
	norm.ID = domain.LoanID(l.st.nextID("loans"))
	l.st.Loans[norm.ID] = norm
	return norm, nil
}

func (l loans) Update(_ context.Context, loan domain.Loan) error {
	if err := l.writable(); err != nil {
		return err
	}
	norm, err := domain.NewLoan(loan)
	if err != nil {
		return err
	}
	if _, ok := l.st.Loans[norm.ID]; !ok {
		return notFound("loan", norm.ID)
	}
	if _, ok := l.st.Accounts[norm.DebitAccount]; !ok {
		return notFound("account", norm.DebitAccount)
	}
	if err := l.checkIdentity(norm); err != nil {
		return err
	}
	l.st.Loans[norm.ID] = norm
	return nil
}

func (l loans) Delete(_ context.Context, id domain.LoanID) error {
	if err := l.writable(); err != nil {
		return err
	}
	if _, ok := l.st.Loans[id]; !ok {
		return notFound("loan", id)
	}
	delete(l.st.Loans, id)
	for key := range maps.Keys(l.st.LoanSchedules) {
		if key.loanID == id {
			delete(l.st.LoanSchedules, key)
		}
	}
	delete(l.st.LoanEvents, id)
	for key := range maps.Keys(l.st.InstallmentLinks) {
		if key.loanID == id {
			delete(l.st.InstallmentLinks, key)
		}
	}
	return nil
}

func (l loans) Get(_ context.Context, id domain.LoanID) (domain.Loan, error) {
	loan, ok := l.st.Loans[id]
	if !ok {
		return domain.Loan{}, notFound("loan", id)
	}
	return loan, nil
}

func (l loans) List(context.Context) ([]domain.Loan, error) {
	return sortedValues(l.st.Loans), nil
}

func (l loans) AddScheduleVersion(_ context.Context, schedule domain.Schedule) error {
	if err := l.writable(); err != nil {
		return err
	}
	loan, ok := l.st.Loans[schedule.LoanID]
	if !ok {
		return notFound("loan", schedule.LoanID)
	}
	if err := validateSchedule(schedule, loan); err != nil {
		return err
	}
	key := loanScheduleKey{loanID: schedule.LoanID, validFrom: schedule.ValidFrom}
	if _, dup := l.st.LoanSchedules[key]; dup {
		return exists("loan schedule", fmt.Sprintf("%d/%s", schedule.LoanID, schedule.ValidFrom))
	}
	l.st.LoanSchedules[key] = cloneSchedule(schedule)
	for _, installment := range schedule.Installments {
		delete(l.st.InstallmentLinks, loanInstallmentKey{
			loanID:       schedule.LoanID,
			scheduleFrom: schedule.ValidFrom,
			due:          installment.Due,
		})
	}
	return nil
}

func (l loans) ScheduleAt(_ context.Context, loanID domain.LoanID, at domain.Date) (domain.Schedule, error) {
	if at.IsZero() {
		return domain.Schedule{}, fmt.Errorf("schedule at: %w", domain.ErrMissingField)
	}
	if _, ok := l.st.Loans[loanID]; !ok {
		return domain.Schedule{}, notFound("loan", loanID)
	}
	var (
		bestKey loanScheduleKey
		found   bool
	)
	for key := range maps.Keys(l.st.LoanSchedules) {
		if key.loanID != loanID || key.validFrom.After(at) {
			continue
		}
		if !found || key.validFrom.After(bestKey.validFrom) {
			bestKey = key
			found = true
		}
	}
	if !found {
		return domain.Schedule{}, notFound("loan schedule", fmt.Sprintf("%d@%s", loanID, at))
	}
	return cloneSchedule(l.st.LoanSchedules[bestKey]), nil
}

func (l loans) InsertEvent(_ context.Context, event domain.LoanEvent) (domain.LoanEvent, error) {
	if err := l.writable(); err != nil {
		return domain.LoanEvent{}, err
	}
	loan, ok := l.st.Loans[event.LoanID]
	if !ok {
		return domain.LoanEvent{}, notFound("loan", event.LoanID)
	}
	if err := validateEvent(event, loan.Principal.Currency()); err != nil {
		return domain.LoanEvent{}, err
	}
	out := cloneEvent(event)
	l.st.LoanEvents[event.LoanID] = append(l.st.LoanEvents[event.LoanID], out)
	return out, nil
}

func (l loans) ListEvents(_ context.Context, loanID domain.LoanID) ([]domain.LoanEvent, error) {
	if _, ok := l.st.Loans[loanID]; !ok {
		return nil, notFound("loan", loanID)
	}
	events := slices.Clone(l.st.LoanEvents[loanID])
	slices.SortStableFunc(events, func(a, b domain.LoanEvent) int {
		return cmp.Compare(a.EventDate.String(), b.EventDate.String())
	})
	for i := range events {
		events[i] = cloneEvent(events[i])
	}
	return events, nil
}

func (l loans) LinkInstallment(
	_ context.Context, loanID domain.LoanID, scheduleFrom, due domain.Date, txID domain.TransactionID,
) error {
	if err := l.writable(); err != nil {
		return err
	}
	if _, ok := l.st.Txs[txID]; !ok {
		return notFound("transaction", txID)
	}
	key := loanInstallmentKey{loanID: loanID, scheduleFrom: scheduleFrom, due: due}
	if err := l.ensureInstallmentKey(key); err != nil {
		return err
	}
	l.st.InstallmentLinks[key] = txID
	return nil
}

func (l loans) UnlinkInstallment(_ context.Context, loanID domain.LoanID, scheduleFrom, due domain.Date) error {
	if err := l.writable(); err != nil {
		return err
	}
	key := loanInstallmentKey{loanID: loanID, scheduleFrom: scheduleFrom, due: due}
	if err := l.ensureInstallmentKey(key); err != nil {
		return err
	}
	delete(l.st.InstallmentLinks, key)
	return nil
}

func (l loans) TransactionOfInstallment(
	_ context.Context, loanID domain.LoanID, scheduleFrom, due domain.Date,
) (domain.TransactionID, error) {
	key := loanInstallmentKey{loanID: loanID, scheduleFrom: scheduleFrom, due: due}
	if err := l.ensureInstallmentKey(key); err != nil {
		return 0, err
	}
	id, ok := l.st.InstallmentLinks[key]
	if !ok {
		return 0, notFound("loan installment transaction", fmt.Sprintf("%d/%s/%s", loanID, scheduleFrom, due))
	}
	return id, nil
}

func (l loans) InstallmentOfTransaction(_ context.Context, txID domain.TransactionID) (app.InstallmentRef, error) {
	var (
		best  loanInstallmentKey
		found bool
	)
	for key, id := range l.st.InstallmentLinks {
		// Same tie-break as the SQL: newest schedule version first.
		if id == txID && (!found || key.scheduleFrom.After(best.scheduleFrom)) {
			best, found = key, true
		}
	}
	if !found {
		return app.InstallmentRef{}, notFound("installment of transaction", txID)
	}
	return app.InstallmentRef{LoanID: best.loanID, ScheduleFrom: best.scheduleFrom, Due: best.due}, nil
}

func (l loans) ensureInstallmentKey(key loanInstallmentKey) error {
	if key.scheduleFrom.IsZero() || key.due.IsZero() {
		return fmt.Errorf("loan installment key: %w", domain.ErrMissingField)
	}
	schedule, ok := l.st.LoanSchedules[loanScheduleKey{loanID: key.loanID, validFrom: key.scheduleFrom}]
	if !ok {
		return notFound("loan schedule", fmt.Sprintf("%d/%s", key.loanID, key.scheduleFrom))
	}
	if !slices.ContainsFunc(schedule.Installments, func(inst domain.Installment) bool { return inst.Due == key.due }) {
		return notFound("loan installment", fmt.Sprintf("%d/%s/%s", key.loanID, key.scheduleFrom, key.due))
	}
	return nil
}

func validateSchedule(schedule domain.Schedule, loan domain.Loan) error {
	if schedule.LoanID != loan.ID {
		return fmt.Errorf("loan schedule %d on loan %d: %w", schedule.LoanID, loan.ID, domain.ErrInvalidValue)
	}
	if schedule.ValidFrom.IsZero() {
		return fmt.Errorf("loan schedule valid_from: %w", domain.ErrMissingField)
	}
	if !schedule.Origin.Valid() {
		return fmt.Errorf("loan schedule origin %q: %w", schedule.Origin, domain.ErrInvalidValue)
	}
	if _, err := domain.NewAnnualRate(schedule.Rate.BasisPoints()); err != nil {
		return err
	}
	for _, inst := range schedule.Installments {
		if inst.Due.IsZero() {
			return fmt.Errorf("loan installment due_date: %w", domain.ErrMissingField)
		}
		for _, m := range []domain.Money{inst.Principal, inst.Interest, inst.Fees, inst.Remaining} {
			if m.Currency() != loan.Principal.Currency() {
				return fmt.Errorf("loan installment currency mismatch: %w", domain.ErrCurrencyMismatch)
			}
			if m.Minor() < 0 {
				return fmt.Errorf("loan installment negative amount: %w", domain.ErrInvalidValue)
			}
		}
	}
	return nil
}

func validateEvent(event domain.LoanEvent, cur domain.Currency) error {
	if !event.Kind.Valid() {
		return fmt.Errorf("loan event kind %q: %w", event.Kind, domain.ErrInvalidValue)
	}
	if event.EventDate.IsZero() {
		return fmt.Errorf("loan event date: %w", domain.ErrMissingField)
	}
	if event.NewRate != nil {
		if _, err := domain.NewAnnualRate(event.NewRate.BasisPoints()); err != nil {
			return err
		}
	}
	if event.NewTermMonths != nil && *event.NewTermMonths < 1 {
		return fmt.Errorf("loan event new term months: %w", domain.ErrInvalidValue)
	}
	if event.PrepaidAmount != nil {
		if event.PrepaidAmount.Currency() != cur {
			return fmt.Errorf("loan event prepaid currency: %w", domain.ErrCurrencyMismatch)
		}
		if event.PrepaidAmount.Minor() <= 0 {
			return fmt.Errorf("loan event prepaid amount: %w", domain.ErrInvalidValue)
		}
	}
	if event.PrepayMode != nil && !event.PrepayMode.Valid() {
		return fmt.Errorf("loan event prepay_mode %q: %w", *event.PrepayMode, domain.ErrInvalidValue)
	}
	return nil
}

var _ app.LoanRepo = loans{}
