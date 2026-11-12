package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	gen "github.com/0xballo/fui/internal/store/sqlite/sqlc"
)

type reconciliationRepo struct {
	repos *repos
}

func normalizeReconciliationOpen(in app.Reconciliation) (app.Reconciliation, error) {
	if in.AccountID == 0 {
		return app.Reconciliation{}, fmt.Errorf("reconciliation account: %w", domain.ErrMissingField)
	}
	if in.Date.IsZero() {
		return app.Reconciliation{}, fmt.Errorf("reconciliation date: %w", domain.ErrMissingField)
	}
	if in.StatementBalance.Currency().IsZero() {
		return app.Reconciliation{}, fmt.Errorf("reconciliation statement balance: %w", domain.ErrMissingField)
	}
	in.ClosedAt = time.Time{}
	return in, nil
}

func (r reconciliationRepo) Open(ctx context.Context, in app.Reconciliation) (app.Reconciliation, error) {
	if err := r.repos.writable(); err != nil {
		return app.Reconciliation{}, err
	}
	norm, err := normalizeReconciliationOpen(in)
	if err != nil {
		return app.Reconciliation{}, err
	}
	if _, err := r.repos.queries.GetAccount(ctx, int64(norm.AccountID)); err != nil {
		return app.Reconciliation{}, mapNotFound(mapSQLError(err), fmt.Sprintf("account %d", norm.AccountID))
	}
	row, err := r.repos.queries.InsertReconciliation(ctx, gen.InsertReconciliationParams{
		AccountID:             int64(norm.AccountID),
		Date:                  norm.Date.String(),
		StatementBalanceMinor: norm.StatementBalance.Minor(),
		Currency:              norm.StatementBalance.Currency().Code(),
		ClosedAt:              sql.NullString{},
	})
	if err != nil {
		return app.Reconciliation{}, mapConstraint(err)
	}
	return toReconciliation(row)
}

func (r reconciliationRepo) Close(ctx context.Context, accountID domain.AccountID, date domain.Date, closedAt time.Time) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	if accountID == 0 {
		return fmt.Errorf("reconciliation account: %w", domain.ErrMissingField)
	}
	if date.IsZero() {
		return fmt.Errorf("reconciliation date: %w", domain.ErrMissingField)
	}
	if closedAt.IsZero() {
		return fmt.Errorf("reconciliation closed_at: %w", domain.ErrMissingField)
	}
	changed, err := r.repos.queries.CloseReconciliation(ctx, gen.CloseReconciliationParams{
		AccountID: int64(accountID),
		Date:      date.String(),
		ClosedAt:  toNullString(closedAt.UTC().Format(time.RFC3339Nano)),
	})
	if err != nil {
		return mapSQLError(err)
	}
	if changed == 0 {
		return fmt.Errorf("reconciliation %d/%s: %w", accountID, date, domain.ErrNotFound)
	}
	return nil
}

func (r reconciliationRepo) Get(ctx context.Context, accountID domain.AccountID, date domain.Date) (app.Reconciliation, error) {
	row, err := r.repos.queries.GetReconciliation(ctx, gen.GetReconciliationParams{
		AccountID: int64(accountID),
		Date:      date.String(),
	})
	if err != nil {
		return app.Reconciliation{}, mapNotFound(mapSQLError(err), fmt.Sprintf("reconciliation %d/%s", accountID, date))
	}
	return toReconciliation(row)
}

func (r reconciliationRepo) LastClosed(ctx context.Context, accountID domain.AccountID) (app.Reconciliation, error) {
	row, err := r.repos.queries.LastClosedReconciliation(ctx, int64(accountID))
	if err != nil {
		return app.Reconciliation{}, mapNotFound(mapSQLError(err), fmt.Sprintf("reconciliation account %d", accountID))
	}
	return toReconciliation(row)
}

func toReconciliation(row gen.Reconciliation) (app.Reconciliation, error) {
	date, err := domain.ParseDate(row.Date)
	if err != nil {
		return app.Reconciliation{}, fmt.Errorf("parse reconciliation date %q: %w", row.Date, err)
	}
	cur, err := domain.ParseCurrency(row.Currency)
	if err != nil {
		return app.Reconciliation{}, fmt.Errorf("parse reconciliation currency %q: %w", row.Currency, err)
	}
	statementBalance, err := domain.NewMoney(row.StatementBalanceMinor, cur)
	if err != nil {
		return app.Reconciliation{}, fmt.Errorf("parse reconciliation statement balance: %w", err)
	}
	out := app.Reconciliation{
		ID:               app.ReconciliationID(row.ID),
		AccountID:        domain.AccountID(row.AccountID),
		Date:             date,
		StatementBalance: statementBalance,
	}
	if row.ClosedAt.Valid {
		out.ClosedAt, err = time.Parse(time.RFC3339Nano, row.ClosedAt.String)
		if err != nil {
			return app.Reconciliation{}, fmt.Errorf("parse reconciliation closed_at %q: %w", row.ClosedAt.String, err)
		}
		out.ClosedAt = out.ClosedAt.UTC()
	}
	return out, nil
}

var _ app.ReconciliationRepo = reconciliationRepo{}
