package sqlite

import (
	"context"
	"fmt"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	gen "github.com/0xballo/fui/internal/store/sqlite/sqlc"
)

type splitRepo struct {
	repos *repos
}

func (r splitRepo) transaction(ctx context.Context, id domain.TransactionID) (domain.Transaction, error) {
	row, err := r.repos.queries.GetTransaction(ctx, int64(id))
	if err != nil {
		return domain.Transaction{}, mapNotFound(mapSQLError(err), fmt.Sprintf("transaction %d", id))
	}
	return toTransaction(row)
}

func (r splitRepo) SplitsOf(ctx context.Context, id domain.TransactionID) ([]domain.Split, error) {
	tx, err := r.transaction(ctx, id)
	if err != nil {
		return nil, err
	}
	rows, err := r.repos.queries.SplitsOfTransaction(ctx, int64(id))
	if err != nil {
		return nil, mapSQLError(err)
	}
	out := make([]domain.Split, 0, len(rows))
	for _, row := range rows {
		amount, err := domain.NewMoney(row.AmountMinor, tx.Amount.Currency())
		if err != nil {
			return nil, fmt.Errorf("split amount of transaction %d: %w", id, err)
		}
		out = append(out, domain.Split{Category: domain.CategoryID(row.CategoryID), Amount: amount, Note: row.Note})
	}
	return out, nil
}

func (r splitRepo) ReplaceSplits(ctx context.Context, id domain.TransactionID, rows []domain.Split) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	tx, err := r.transaction(ctx, id)
	if err != nil {
		return err
	}
	if len(rows) > 0 {
		if err := domain.ValidateSplits(tx.Amount, rows); err != nil {
			return fmt.Errorf("transaction %d: %w", id, err)
		}
	}
	if err := r.repos.queries.ClearTransactionSplits(ctx, int64(id)); err != nil {
		return mapSQLError(err)
	}
	for _, row := range rows {
		if _, err := r.repos.queries.GetCategory(ctx, int64(row.Category)); err != nil {
			return mapNotFound(mapSQLError(err), fmt.Sprintf("split category %d", row.Category))
		}
		if err := r.repos.queries.AddTransactionSplit(ctx, gen.AddTransactionSplitParams{
			TransactionID: int64(id),
			CategoryID:    int64(row.Category),
			AmountMinor:   row.Amount.Minor(),
			Note:          row.Note,
		}); err != nil {
			return mapConstraint(err)
		}
	}
	return nil
}

var _ app.SplitRepo = splitRepo{}
