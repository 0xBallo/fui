package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	gen "github.com/0xballo/fui/internal/store/sqlite/sqlc"
)

type loanRepo struct {
	repos *repos
}

func (r loanRepo) Insert(ctx context.Context, loan domain.Loan) (domain.Loan, error) {
	if err := r.repos.writable(); err != nil {
		return domain.Loan{}, err
	}
	norm, err := domain.NewLoan(loan)
	if err != nil {
		return domain.Loan{}, err
	}
	if _, err := r.repos.queries.GetAccount(ctx, int64(norm.DebitAccount)); err != nil {
		return domain.Loan{}, mapNotFound(mapSQLError(err), "loan debit account")
	}
	row, err := r.repos.queries.InsertLoan(ctx, gen.InsertLoanParams{
		Kind:           string(norm.Kind),
		PrincipalMinor: norm.Principal.Minor(),
		Currency:       norm.Principal.Currency().Code(),
		DisbursedOn:    norm.DisbursedOn.String(),
		TermMonths:     int64(norm.TermMonths),
		RateType:       string(norm.RateType),
		DebitAccountID: int64(norm.DebitAccount),
	})
	if err != nil {
		return domain.Loan{}, mapConstraint(err)
	}
	return toLoan(row)
}

func (r loanRepo) Update(ctx context.Context, loan domain.Loan) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	norm, err := domain.NewLoan(loan)
	if err != nil {
		return err
	}
	changed, err := r.repos.queries.UpdateLoan(ctx, gen.UpdateLoanParams{
		ID:             int64(norm.ID),
		Kind:           string(norm.Kind),
		PrincipalMinor: norm.Principal.Minor(),
		Currency:       norm.Principal.Currency().Code(),
		DisbursedOn:    norm.DisbursedOn.String(),
		TermMonths:     int64(norm.TermMonths),
		RateType:       string(norm.RateType),
		DebitAccountID: int64(norm.DebitAccount),
	})
	if err != nil {
		return mapConstraint(err)
	}
	if changed == 0 {
		return fmt.Errorf("loan %d: %w", norm.ID, domain.ErrNotFound)
	}
	return nil
}

func (r loanRepo) Delete(ctx context.Context, id domain.LoanID) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	changed, err := r.repos.queries.DeleteLoan(ctx, int64(id))
	if err != nil {
		return mapSQLError(err)
	}
	if changed == 0 {
		return fmt.Errorf("loan %d: %w", id, domain.ErrNotFound)
	}
	return nil
}

func (r loanRepo) Get(ctx context.Context, id domain.LoanID) (domain.Loan, error) {
	row, err := r.repos.queries.GetLoan(ctx, int64(id))
	if err != nil {
		return domain.Loan{}, mapNotFound(mapSQLError(err), "get loan")
	}
	return toLoan(row)
}

func (r loanRepo) List(ctx context.Context) ([]domain.Loan, error) {
	rows, err := r.repos.queries.ListLoans(ctx)
	if err != nil {
		return nil, mapSQLError(err)
	}
	out := make([]domain.Loan, 0, len(rows))
	for _, row := range rows {
		loan, convErr := toLoan(row)
		if convErr != nil {
			return nil, convErr
		}
		out = append(out, loan)
	}
	return out, nil
}

func (r loanRepo) AddScheduleVersion(ctx context.Context, schedule domain.Schedule) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	loan, err := r.Get(ctx, schedule.LoanID)
	if err != nil {
		return err
	}
	if err := validateScheduleVersion(schedule, loan); err != nil {
		return err
	}

	sRow, err := r.repos.queries.InsertLoanSchedule(ctx, gen.InsertLoanScheduleParams{
		LoanID:          int64(schedule.LoanID),
		ValidFrom:       schedule.ValidFrom.String(),
		Origin:          string(schedule.Origin),
		RateBasisPoints: schedule.Rate.BasisPoints(),
	})
	if err != nil {
		return mapConstraint(err)
	}
	for _, inst := range schedule.Installments {
		if err := r.repos.queries.InsertLoanInstallment(ctx, gen.InsertLoanInstallmentParams{
			ScheduleID:     sRow.ID,
			DueDate:        inst.Due.String(),
			PrincipalMinor: inst.Principal.Minor(),
			InterestMinor:  inst.Interest.Minor(),
			FeesMinor:      inst.Fees.Minor(),
			RemainingMinor: inst.Remaining.Minor(),
			TransactionID:  sql.NullInt64{},
		}); err != nil {
			return mapConstraint(err)
		}
	}
	return nil
}

func (r loanRepo) ScheduleAt(ctx context.Context, loanID domain.LoanID, at domain.Date) (domain.Schedule, error) {
	loan, err := r.Get(ctx, loanID)
	if err != nil {
		return domain.Schedule{}, err
	}
	if at.IsZero() {
		return domain.Schedule{}, fmt.Errorf("schedule at: %w", domain.ErrMissingField)
	}
	sRow, err := r.repos.queries.GetScheduleAt(ctx, gen.GetScheduleAtParams{
		LoanID: int64(loanID),
		AtDate: at.String(),
	})
	if err != nil {
		return domain.Schedule{}, mapNotFound(mapSQLError(err), "schedule at")
	}
	iRows, err := r.repos.queries.ListLoanInstallmentsBySchedule(ctx, sRow.ID)
	if err != nil {
		return domain.Schedule{}, mapSQLError(err)
	}
	return toSchedule(sRow, iRows, loan.Principal.Currency())
}

func (r loanRepo) InsertEvent(ctx context.Context, event domain.LoanEvent) (domain.LoanEvent, error) {
	if err := r.repos.writable(); err != nil {
		return domain.LoanEvent{}, err
	}
	loan, err := r.Get(ctx, event.LoanID)
	if err != nil {
		return domain.LoanEvent{}, err
	}
	params, err := toInsertLoanEventParams(event, loan.Principal.Currency())
	if err != nil {
		return domain.LoanEvent{}, err
	}
	row, err := r.repos.queries.InsertLoanEvent(ctx, params)
	if err != nil {
		return domain.LoanEvent{}, mapConstraint(err)
	}
	return toLoanEvent(row, loan.Principal.Currency())
}

func (r loanRepo) ListEvents(ctx context.Context, loanID domain.LoanID) ([]domain.LoanEvent, error) {
	loan, err := r.Get(ctx, loanID)
	if err != nil {
		return nil, err
	}
	rows, err := r.repos.queries.ListLoanEvents(ctx, int64(loanID))
	if err != nil {
		return nil, mapSQLError(err)
	}
	out := make([]domain.LoanEvent, 0, len(rows))
	for _, row := range rows {
		event, convErr := toLoanEvent(row, loan.Principal.Currency())
		if convErr != nil {
			return nil, convErr
		}
		out = append(out, event)
	}
	return out, nil
}

func (r loanRepo) LinkInstallment(
	ctx context.Context, loanID domain.LoanID, scheduleFrom, due domain.Date, txID domain.TransactionID,
) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	if scheduleFrom.IsZero() || due.IsZero() {
		return fmt.Errorf("link loan installment: %w", domain.ErrMissingField)
	}
	if _, err := r.repos.queries.GetTransaction(ctx, int64(txID)); err != nil {
		return mapNotFound(mapSQLError(err), "loan installment transaction")
	}
	changed, err := r.repos.queries.LinkLoanInstallment(ctx, gen.LinkLoanInstallmentParams{
		LoanID:       int64(loanID),
		ScheduleFrom: scheduleFrom.String(),
		DueDate:      due.String(),
		TransactionID: sql.NullInt64{
			Int64: int64(txID),
			Valid: true,
		},
	})
	if err != nil {
		return mapConstraint(err)
	}
	if changed == 0 {
		return fmt.Errorf("loan installment %d %s/%s: %w", loanID, scheduleFrom, due, domain.ErrNotFound)
	}
	return nil
}

func (r loanRepo) UnlinkInstallment(ctx context.Context, loanID domain.LoanID, scheduleFrom, due domain.Date) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	if scheduleFrom.IsZero() || due.IsZero() {
		return fmt.Errorf("unlink loan installment: %w", domain.ErrMissingField)
	}
	changed, err := r.repos.queries.UnlinkLoanInstallment(ctx, gen.UnlinkLoanInstallmentParams{
		LoanID:       int64(loanID),
		ScheduleFrom: scheduleFrom.String(),
		DueDate:      due.String(),
	})
	if err != nil {
		return mapSQLError(err)
	}
	if changed == 0 {
		return fmt.Errorf("loan installment %d %s/%s: %w", loanID, scheduleFrom, due, domain.ErrNotFound)
	}
	return nil
}

func toLoan(row gen.Loan) (domain.Loan, error) {
	kind, err := domain.ParseLoanKind(row.Kind)
	if err != nil {
		return domain.Loan{}, fmt.Errorf("parse loan kind %q: %w", row.Kind, err)
	}
	rateType, err := domain.ParseRateType(row.RateType)
	if err != nil {
		return domain.Loan{}, fmt.Errorf("parse loan rate_type %q: %w", row.RateType, err)
	}
	cur, err := domain.ParseCurrency(row.Currency)
	if err != nil {
		return domain.Loan{}, fmt.Errorf("parse loan currency %q: %w", row.Currency, err)
	}
	principal, err := domain.NewMoney(row.PrincipalMinor, cur)
	if err != nil {
		return domain.Loan{}, fmt.Errorf("parse loan principal: %w", err)
	}
	disbursedOn, err := domain.ParseDate(row.DisbursedOn)
	if err != nil {
		return domain.Loan{}, fmt.Errorf("parse loan disbursed_on %q: %w", row.DisbursedOn, err)
	}
	return domain.NewLoan(domain.Loan{
		ID:           domain.LoanID(row.ID),
		Kind:         kind,
		Principal:    principal,
		DisbursedOn:  disbursedOn,
		TermMonths:   int(row.TermMonths),
		RateType:     rateType,
		DebitAccount: domain.AccountID(row.DebitAccountID),
	})
}

func toSchedule(row gen.LoanSchedule, installments []gen.LoanInstallment, cur domain.Currency) (domain.Schedule, error) {
	validFrom, err := domain.ParseDate(row.ValidFrom)
	if err != nil {
		return domain.Schedule{}, fmt.Errorf("parse schedule valid_from %q: %w", row.ValidFrom, err)
	}
	origin := domain.ScheduleOrigin(row.Origin)
	if !origin.Valid() {
		return domain.Schedule{}, fmt.Errorf("parse schedule origin %q: %w", row.Origin, domain.ErrInvalidValue)
	}
	rate, err := domain.NewAnnualRate(row.RateBasisPoints)
	if err != nil {
		return domain.Schedule{}, fmt.Errorf("parse schedule rate: %w", err)
	}
	out := domain.Schedule{
		LoanID:       domain.LoanID(row.LoanID),
		ValidFrom:    validFrom,
		Origin:       origin,
		Rate:         rate,
		Installments: make([]domain.Installment, 0, len(installments)),
	}
	for _, inst := range installments {
		due, err := domain.ParseDate(inst.DueDate)
		if err != nil {
			return domain.Schedule{}, fmt.Errorf("parse installment due_date %q: %w", inst.DueDate, err)
		}
		principal, err := domain.NewMoney(inst.PrincipalMinor, cur)
		if err != nil {
			return domain.Schedule{}, fmt.Errorf("parse installment principal: %w", err)
		}
		interest, err := domain.NewMoney(inst.InterestMinor, cur)
		if err != nil {
			return domain.Schedule{}, fmt.Errorf("parse installment interest: %w", err)
		}
		fees, err := domain.NewMoney(inst.FeesMinor, cur)
		if err != nil {
			return domain.Schedule{}, fmt.Errorf("parse installment fees: %w", err)
		}
		remaining, err := domain.NewMoney(inst.RemainingMinor, cur)
		if err != nil {
			return domain.Schedule{}, fmt.Errorf("parse installment remaining: %w", err)
		}
		out.Installments = append(out.Installments, domain.Installment{
			Due:       due,
			Principal: principal,
			Interest:  interest,
			Fees:      fees,
			Remaining: remaining,
		})
	}
	return out, nil
}

func validateScheduleVersion(schedule domain.Schedule, loan domain.Loan) error {
	if schedule.LoanID != loan.ID {
		return fmt.Errorf("loan schedule loan %d: %w", schedule.LoanID, domain.ErrInvalidValue)
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
		if inst.Principal.Currency() != loan.Principal.Currency() ||
			inst.Interest.Currency() != loan.Principal.Currency() ||
			inst.Fees.Currency() != loan.Principal.Currency() ||
			inst.Remaining.Currency() != loan.Principal.Currency() {
			return fmt.Errorf("loan installment currency mismatch: %w", domain.ErrCurrencyMismatch)
		}
		if inst.Principal.Minor() < 0 || inst.Interest.Minor() < 0 || inst.Fees.Minor() < 0 || inst.Remaining.Minor() < 0 {
			return fmt.Errorf("loan installment negative amount: %w", domain.ErrInvalidValue)
		}
	}
	return nil
}

func toInsertLoanEventParams(event domain.LoanEvent, cur domain.Currency) (gen.InsertLoanEventParams, error) {
	if !event.Kind.Valid() {
		return gen.InsertLoanEventParams{}, fmt.Errorf("loan event kind %q: %w", event.Kind, domain.ErrInvalidValue)
	}
	if event.EventDate.IsZero() {
		return gen.InsertLoanEventParams{}, fmt.Errorf("loan event date: %w", domain.ErrMissingField)
	}
	params := gen.InsertLoanEventParams{
		LoanID:             int64(event.LoanID),
		Kind:               string(event.Kind),
		EventDate:          event.EventDate.String(),
		NewRateBasisPoints: sql.NullInt64{},
		NewTermMonths:      sql.NullInt64{},
		PrepaidMinor:       sql.NullInt64{},
		PrepaidCurrency:    sql.NullString{},
		PrepayMode:         sql.NullString{},
	}
	if event.NewRate != nil {
		params.NewRateBasisPoints = sql.NullInt64{Int64: event.NewRate.BasisPoints(), Valid: true}
	}
	if event.NewTermMonths != nil {
		params.NewTermMonths = sql.NullInt64{Int64: int64(*event.NewTermMonths), Valid: true}
	}
	if event.PrepaidAmount != nil {
		if event.PrepaidAmount.Currency() != cur {
			return gen.InsertLoanEventParams{}, fmt.Errorf("loan event prepaid amount currency: %w", domain.ErrCurrencyMismatch)
		}
		params.PrepaidMinor = sql.NullInt64{Int64: event.PrepaidAmount.Minor(), Valid: true}
		params.PrepaidCurrency = sql.NullString{String: event.PrepaidAmount.Currency().Code(), Valid: true}
	}
	if event.PrepayMode != nil {
		params.PrepayMode = sql.NullString{String: string(*event.PrepayMode), Valid: true}
	}
	return params, nil
}

func toLoanEvent(row gen.LoanEvent, cur domain.Currency) (domain.LoanEvent, error) {
	eventDate, err := domain.ParseDate(row.EventDate)
	if err != nil {
		return domain.LoanEvent{}, fmt.Errorf("parse loan event date %q: %w", row.EventDate, err)
	}
	out := domain.LoanEvent{
		LoanID:    domain.LoanID(row.LoanID),
		Kind:      domain.LoanEventKind(row.Kind),
		EventDate: eventDate,
	}
	if !out.Kind.Valid() {
		return domain.LoanEvent{}, fmt.Errorf("parse loan event kind %q: %w", row.Kind, domain.ErrInvalidValue)
	}
	if row.NewRateBasisPoints.Valid {
		rate, err := domain.NewAnnualRate(row.NewRateBasisPoints.Int64)
		if err != nil {
			return domain.LoanEvent{}, fmt.Errorf("parse loan event rate: %w", err)
		}
		out.NewRate = &rate
	}
	if row.NewTermMonths.Valid {
		term := int(row.NewTermMonths.Int64)
		out.NewTermMonths = &term
	}
	if row.PrepaidMinor.Valid {
		amount, err := domain.NewMoney(row.PrepaidMinor.Int64, cur)
		if err != nil {
			return domain.LoanEvent{}, fmt.Errorf("parse loan event prepaid amount: %w", err)
		}
		out.PrepaidAmount = &amount
	}
	if row.PrepayMode.Valid {
		mode := domain.PrepaymentMode(row.PrepayMode.String)
		if !mode.Valid() {
			return domain.LoanEvent{}, fmt.Errorf("parse loan event prepay_mode %q: %w", row.PrepayMode.String, domain.ErrInvalidValue)
		}
		out.PrepayMode = &mode
	}
	return out, nil
}

var _ app.LoanRepo = loanRepo{}
