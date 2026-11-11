package sqlite

import (
	"context"
	"fmt"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	gen "github.com/0xballo/fui/internal/store/sqlite/sqlc"
)

type transactionRepo struct {
	repos *repos
}

func (r transactionRepo) Insert(ctx context.Context, t domain.Transaction) (domain.Transaction, error) {
	if err := r.repos.writable(); err != nil {
		return domain.Transaction{}, err
	}
	if _, err := r.repos.queries.GetAccount(ctx, int64(t.AccountID)); err != nil {
		return domain.Transaction{}, mapNotFound(mapSQLError(err), "transaction account")
	}
	row, err := r.repos.queries.InsertTransaction(ctx, toInsertTransactionParams(t))
	if err != nil {
		return domain.Transaction{}, mapConstraint(err)
	}
	return toTransaction(row)
}

func (r transactionRepo) Update(ctx context.Context, t domain.Transaction) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	params := toUpdateTransactionParams(t)
	changed, err := r.repos.queries.UpdateTransaction(ctx, params)
	if err != nil {
		return mapConstraint(err)
	}
	if changed == 0 {
		return fmt.Errorf("transaction %d: %w", t.ID, domain.ErrNotFound)
	}
	return nil
}

func (r transactionRepo) Delete(ctx context.Context, id domain.TransactionID) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	changed, err := r.repos.queries.DeleteTransaction(ctx, int64(id))
	if err != nil {
		return mapSQLError(err)
	}
	if changed == 0 {
		return fmt.Errorf("transaction %d: %w", id, domain.ErrNotFound)
	}
	return nil
}

func (r transactionRepo) Get(ctx context.Context, id domain.TransactionID) (domain.Transaction, error) {
	row, err := r.repos.queries.GetTransaction(ctx, int64(id))
	if err != nil {
		return domain.Transaction{}, mapNotFound(mapSQLError(err), "get transaction")
	}
	return toTransaction(row)
}

func (r transactionRepo) List(ctx context.Context, f app.TxFilter, after *app.TxCursor, limit int) ([]domain.Transaction, error) {
	params, err := listTransactionParams(f, after, limit)
	if err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}
	rows, err := r.repos.queries.ListTransactions(ctx, params)
	if err != nil {
		return nil, mapSQLError(err)
	}
	return mapTransactions(rows)
}

func (r transactionRepo) SumUpTo(ctx context.Context, id domain.AccountID, date domain.Date) (int64, error) {
	sum, err := r.repos.queries.SumTransactionsUpTo(ctx, gen.SumTransactionsUpToParams{
		AccountID: int64(id),
		Date:      date.String(),
	})
	if err != nil {
		return 0, mapSQLError(err)
	}
	return sum, nil
}

func (r transactionRepo) ByExternalID(ctx context.Context, id domain.AccountID, externalID string) ([]domain.Transaction, error) {
	rows, err := r.repos.queries.TransactionsByExternalID(ctx, gen.TransactionsByExternalIDParams{
		AccountID:  int64(id),
		ExternalID: toNullString(externalID),
	})
	if err != nil {
		return nil, mapSQLError(err)
	}
	return mapTransactions(rows)
}

func (r transactionRepo) ByFingerprint(ctx context.Context, fingerprint string) ([]domain.Transaction, error) {
	rows, err := r.repos.queries.TransactionsByFingerprint(ctx, fingerprint)
	if err != nil {
		return nil, mapSQLError(err)
	}
	return mapTransactions(rows)
}

func (r transactionRepo) NearAmount(ctx context.Context, id domain.AccountID, minor int64, from, to domain.Date) ([]domain.Transaction, error) {
	rows, err := r.repos.queries.TransactionsNearAmount(ctx, gen.TransactionsNearAmountParams{
		AccountID:   int64(id),
		AmountMinor: minor,
		FromDate:    from.String(),
		ToDate:      to.String(),
	})
	if err != nil {
		return nil, mapSQLError(err)
	}
	return mapTransactions(rows)
}

func (r transactionRepo) ByTransferGroup(ctx context.Context, id domain.TransferGroupID) ([]domain.Transaction, error) {
	rows, err := r.repos.queries.TransactionsByTransferGroup(ctx, toNullInt64(id))
	if err != nil {
		return nil, mapSQLError(err)
	}
	return mapTransactions(rows)
}

func toInsertTransactionParams(t domain.Transaction) gen.InsertTransactionParams {
	return gen.InsertTransactionParams{
		AccountID:       int64(t.AccountID),
		Date:            t.Date.String(),
		OperationDate:   toNullDate(t.OperationDate),
		ValueDate:       toNullDate(t.ValueDate),
		AmountMinor:     t.Amount.Minor(),
		Currency:        t.Amount.Currency().Code(),
		PayeeID:         toNullInt64(t.PayeeID),
		Description:     t.Description,
		CategoryID:      toNullInt64(t.CategoryID),
		Notes:           t.Notes,
		Status:          string(t.Status),
		Kind:            string(t.Kind),
		ExternalID:      toNullString(t.ExternalID),
		Fingerprint:     t.Fingerprint,
		ImportBatchID:   toNullInt64(t.ImportBatchID),
		TransferGroupID: toNullInt64(t.TransferGroupID),
	}
}

func toUpdateTransactionParams(t domain.Transaction) gen.UpdateTransactionParams {
	params := toInsertTransactionParams(t)
	return gen.UpdateTransactionParams{
		ID:              int64(t.ID),
		AccountID:       params.AccountID,
		Date:            params.Date,
		OperationDate:   params.OperationDate,
		ValueDate:       params.ValueDate,
		AmountMinor:     params.AmountMinor,
		Currency:        params.Currency,
		PayeeID:         params.PayeeID,
		Description:     params.Description,
		CategoryID:      params.CategoryID,
		Notes:           params.Notes,
		Status:          params.Status,
		Kind:            params.Kind,
		ExternalID:      params.ExternalID,
		Fingerprint:     params.Fingerprint,
		ImportBatchID:   params.ImportBatchID,
		TransferGroupID: params.TransferGroupID,
	}
}

func listTransactionParams(f app.TxFilter, after *app.TxCursor, limit int) (gen.ListTransactionsParams, error) {
	accountIDsJSON, err := encodeIDSlice(f.AccountIDs)
	if err != nil {
		return gen.ListTransactionsParams{}, err
	}
	categoryIDsJSON, err := encodeIDSlice(f.CategoryIDs)
	if err != nil {
		return gen.ListTransactionsParams{}, err
	}
	statusesJSON, err := encodeStatusSlice(f.Statuses)
	if err != nil {
		return gen.ListTransactionsParams{}, err
	}

	params := gen.ListTransactionsParams{
		FromDate:        dateStringOrEmpty(f.From),
		ToDate:          dateStringOrEmpty(f.To),
		AccountIdsJson:  accountIDsJSON,
		CategoryIdsJson: categoryIDsJSON,
		StatusesJson:    statusesJSON,
		Text:            f.Text,
		MinSet:          int64(0),
		MaxSet:          int64(0),
		ImportBatchID:   int64(f.ImportBatchID),
		AfterSet:        int64(0),
		AfterDate:       "",
		AfterID:         int64(0),
		LimitValue:      int64(limit),
	}
	if f.MinMinor != nil {
		params.MinSet = 1
		params.MinMinor = *f.MinMinor
	}
	if f.MaxMinor != nil {
		params.MaxSet = 1
		params.MaxMinor = *f.MaxMinor
	}
	if after != nil {
		params.AfterSet = 1
		params.AfterDate = after.Date.String()
		params.AfterID = int64(after.ID)
	}
	return params, nil
}

func dateStringOrEmpty(date domain.Date) string {
	if date.IsZero() {
		return ""
	}
	return date.String()
}

func mapTransactions(rows []gen.Transaction) ([]domain.Transaction, error) {
	out := make([]domain.Transaction, 0, len(rows))
	for _, row := range rows {
		tx, err := toTransaction(row)
		if err != nil {
			return nil, err
		}
		out = append(out, tx)
	}
	return out, nil
}
