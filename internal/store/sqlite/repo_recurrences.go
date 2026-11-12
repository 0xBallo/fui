package sqlite

import (
	"context"
	"fmt"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	gen "github.com/0xballo/fui/internal/store/sqlite/sqlc"
)

type recurrenceRepo struct {
	repos *repos
}

func (r recurrenceRepo) Insert(ctx context.Context, in app.Recurrence) (app.Recurrence, error) {
	if err := r.repos.writable(); err != nil {
		return app.Recurrence{}, err
	}
	rec, err := normalizeRecurrence(in)
	if err != nil {
		return app.Recurrence{}, err
	}
	if _, err := r.repos.queries.GetAccount(ctx, int64(rec.AccountID)); err != nil {
		return app.Recurrence{}, mapNotFound(mapSQLError(err), "recurrence account")
	}
	if rec.CategoryID != 0 {
		if _, err := r.repos.queries.GetCategory(ctx, int64(rec.CategoryID)); err != nil {
			return app.Recurrence{}, mapNotFound(mapSQLError(err), "recurrence category")
		}
	}
	row, err := r.repos.queries.InsertRecurrence(ctx, toInsertRecurrenceParams(rec))
	if err != nil {
		return app.Recurrence{}, mapConstraint(err)
	}
	return toRecurrence(row)
}

func (r recurrenceRepo) Update(ctx context.Context, in app.Recurrence) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	if _, err := r.repos.queries.GetRecurrence(ctx, int64(in.ID)); err != nil {
		return mapNotFound(mapSQLError(err), fmt.Sprintf("recurrence %d", in.ID))
	}
	rec, err := normalizeRecurrence(in)
	if err != nil {
		return err
	}
	if _, err := r.repos.queries.GetAccount(ctx, int64(rec.AccountID)); err != nil {
		return mapNotFound(mapSQLError(err), "recurrence account")
	}
	if rec.CategoryID != 0 {
		if _, err := r.repos.queries.GetCategory(ctx, int64(rec.CategoryID)); err != nil {
			return mapNotFound(mapSQLError(err), "recurrence category")
		}
	}
	changed, err := r.repos.queries.UpdateRecurrence(ctx, toUpdateRecurrenceParams(rec))
	if err != nil {
		return mapConstraint(err)
	}
	if changed == 0 {
		return fmt.Errorf("recurrence %d: %w", in.ID, domain.ErrNotFound)
	}
	return nil
}

func (r recurrenceRepo) Delete(ctx context.Context, id app.RecurrenceID) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	changed, err := r.repos.queries.DeleteRecurrence(ctx, int64(id))
	if err != nil {
		return mapSQLError(err)
	}
	if changed == 0 {
		return fmt.Errorf("recurrence %d: %w", id, domain.ErrNotFound)
	}
	return nil
}

func (r recurrenceRepo) Get(ctx context.Context, id app.RecurrenceID) (app.Recurrence, error) {
	row, err := r.repos.queries.GetRecurrence(ctx, int64(id))
	if err != nil {
		return app.Recurrence{}, mapNotFound(mapSQLError(err), fmt.Sprintf("recurrence %d", id))
	}
	return toRecurrence(row)
}

func (r recurrenceRepo) List(ctx context.Context) ([]app.Recurrence, error) {
	rows, err := r.repos.queries.ListRecurrences(ctx)
	if err != nil {
		return nil, mapSQLError(err)
	}
	return mapRecurrences(rows)
}

func (r recurrenceRepo) Due(ctx context.Context, until domain.Date) ([]app.Recurrence, error) {
	if until.IsZero() {
		return nil, fmt.Errorf("due recurrences: %w", domain.ErrMissingField)
	}
	rows, err := r.repos.queries.DueRecurrences(ctx, until.String())
	if err != nil {
		return nil, mapSQLError(err)
	}
	return mapRecurrences(rows)
}

func (r recurrenceRepo) PendingOf(ctx context.Context, id app.RecurrenceID) ([]domain.Transaction, error) {
	if _, err := r.repos.queries.GetRecurrence(ctx, int64(id)); err != nil {
		return nil, mapNotFound(mapSQLError(err), fmt.Sprintf("recurrence %d", id))
	}
	rows, err := r.repos.queries.PendingTransactionsOfRecurrence(ctx, toNullInt64(id))
	if err != nil {
		return nil, mapSQLError(err)
	}
	return mapTransactions(rows)
}

func normalizeRecurrence(in app.Recurrence) (app.Recurrence, error) {
	raw := domain.Recurrence{
		AccountID:   in.AccountID,
		CategoryID:  in.CategoryID,
		Description: in.Description,
		Amount:      in.Amount,
		Rule:        in.Rule,
		Start:       in.Start,
		End:         in.End,
		NextDate:    in.NextDate,
	}
	norm, err := domain.NewRecurrence(raw)
	if err != nil {
		return app.Recurrence{}, err
	}
	in.AccountID = norm.AccountID
	in.CategoryID = norm.CategoryID
	in.Description = norm.Description
	in.Amount = norm.Amount
	in.Rule = norm.Rule
	in.Start = norm.Start
	in.End = norm.End
	in.NextDate = norm.NextDate
	return in, nil
}

func toInsertRecurrenceParams(in app.Recurrence) gen.InsertRecurrenceParams {
	return gen.InsertRecurrenceParams{
		AccountID:   int64(in.AccountID),
		CategoryID:  toNullInt64(in.CategoryID),
		Description: in.Description,
		AmountMinor: in.Amount.Minor(),
		Currency:    in.Amount.Currency().Code(),
		Every:       int64(in.Rule.Every),
		Unit:        string(in.Rule.Unit),
		DayOfMonth:  int64(in.Rule.DayOfMonth),
		StartDate:   in.Start.String(),
		EndDate:     toNullDate(in.End),
		NextDate:    in.NextDate.String(),
	}
}

func toUpdateRecurrenceParams(in app.Recurrence) gen.UpdateRecurrenceParams {
	insert := toInsertRecurrenceParams(in)
	return gen.UpdateRecurrenceParams{
		ID:          int64(in.ID),
		AccountID:   insert.AccountID,
		CategoryID:  insert.CategoryID,
		Description: insert.Description,
		AmountMinor: insert.AmountMinor,
		Currency:    insert.Currency,
		Every:       insert.Every,
		Unit:        insert.Unit,
		DayOfMonth:  insert.DayOfMonth,
		StartDate:   insert.StartDate,
		EndDate:     insert.EndDate,
		NextDate:    insert.NextDate,
	}
}

func toRecurrence(row gen.Recurrence) (app.Recurrence, error) {
	cur, err := domain.ParseCurrency(row.Currency)
	if err != nil {
		return app.Recurrence{}, fmt.Errorf("parse recurrence currency %q: %w", row.Currency, err)
	}
	amount, err := domain.NewMoney(row.AmountMinor, cur)
	if err != nil {
		return app.Recurrence{}, fmt.Errorf("parse recurrence amount: %w", err)
	}
	start, err := domain.ParseDate(row.StartDate)
	if err != nil {
		return app.Recurrence{}, fmt.Errorf("parse recurrence start_date %q: %w", row.StartDate, err)
	}
	nextDate, err := domain.ParseDate(row.NextDate)
	if err != nil {
		return app.Recurrence{}, fmt.Errorf("parse recurrence next_date %q: %w", row.NextDate, err)
	}
	rule, err := domain.NewRecurrenceRule(int(row.Every), domain.RecurrenceUnit(row.Unit), int(row.DayOfMonth))
	if err != nil {
		return app.Recurrence{}, fmt.Errorf("parse recurrence rule: %w", err)
	}
	out := app.Recurrence{
		ID:          app.RecurrenceID(row.ID),
		AccountID:   domain.AccountID(row.AccountID),
		CategoryID:  domain.CategoryID(nullInt64OrZero(row.CategoryID)),
		Description: row.Description,
		Amount:      amount,
		Rule:        rule,
		Start:       start,
		NextDate:    nextDate,
	}
	if row.EndDate.Valid {
		out.End, err = domain.ParseDate(row.EndDate.String)
		if err != nil {
			return app.Recurrence{}, fmt.Errorf("parse recurrence end_date %q: %w", row.EndDate.String, err)
		}
	}
	return normalizeRecurrence(out)
}

func mapRecurrences(rows []gen.Recurrence) ([]app.Recurrence, error) {
	out := make([]app.Recurrence, 0, len(rows))
	for _, row := range rows {
		r, err := toRecurrence(row)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

var _ app.RecurrenceRepo = recurrenceRepo{}
