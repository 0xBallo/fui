package sqlite

import (
	"context"
	"fmt"

	"github.com/0xballo/fui/internal/domain"
	gen "github.com/0xballo/fui/internal/store/sqlite/sqlc"
)

type transferRepo struct {
	repos *repos
}

func (r transferRepo) Insert(ctx context.Context, g domain.TransferGroup) (domain.TransferGroup, error) {
	if err := r.repos.writable(); err != nil {
		return domain.TransferGroup{}, err
	}

	row, err := r.repos.queries.InsertTransferGroup(ctx, gen.InsertTransferGroupParams{
		FeeMinor:    g.Fee.Minor(),
		FeeCurrency: toNullString(g.Fee.Currency().Code()),
	})
	if err != nil {
		return domain.TransferGroup{}, mapConstraint(err)
	}

	inserted := domain.TransferGroup{
		ID: domain.TransferGroupID(row.ID),
	}
	if row.FeeCurrency.Valid {
		cur, parseErr := domain.ParseCurrency(row.FeeCurrency.String)
		if parseErr != nil {
			return domain.TransferGroup{}, parseErr
		}
		fee, moneyErr := domain.NewMoney(row.FeeMinor, cur)
		if moneyErr != nil {
			return domain.TransferGroup{}, moneyErr
		}
		inserted.Fee = fee
	}
	return inserted, nil
}

func (r transferRepo) Get(ctx context.Context, id domain.TransferGroupID) (domain.TransferGroup, error) {
	row, err := r.repos.queries.GetTransferGroup(ctx, int64(id))
	if err != nil {
		return domain.TransferGroup{}, mapNotFound(mapSQLError(err), "get transfer group")
	}
	txs, err := r.repos.queries.TransactionsByTransferGroup(ctx, toNullInt64(id))
	if err != nil {
		return domain.TransferGroup{}, mapSQLError(err)
	}

	group := domain.TransferGroup{ID: domain.TransferGroupID(row.ID)}
	if row.FeeCurrency.Valid {
		cur, parseErr := domain.ParseCurrency(row.FeeCurrency.String)
		if parseErr != nil {
			return domain.TransferGroup{}, parseErr
		}
		fee, moneyErr := domain.NewMoney(row.FeeMinor, cur)
		if moneyErr != nil {
			return domain.TransferGroup{}, moneyErr
		}
		group.Fee = fee
	}

	for _, txRow := range txs {
		tx, convErr := toTransaction(txRow)
		if convErr != nil {
			return domain.TransferGroup{}, convErr
		}
		leg := domain.TransferLeg{
			TransactionID: tx.ID,
			AccountID:     tx.AccountID,
			Amount:        tx.Amount,
		}
		if tx.Amount.Sign() < 0 {
			group.Out = leg
		} else {
			group.In = leg
		}
	}
	return group, nil
}

func (r transferRepo) Delete(ctx context.Context, id domain.TransferGroupID) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	changed, err := r.repos.queries.DeleteTransferGroup(ctx, int64(id))
	if err != nil {
		return mapSQLError(err)
	}
	if changed == 0 {
		return fmt.Errorf("transfer group %d: %w", id, domain.ErrNotFound)
	}
	return nil
}
