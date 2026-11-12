package sqlite

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	gen "github.com/0xballo/fui/internal/store/sqlite/sqlc"
	"github.com/0xballo/fui/internal/textnorm"
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
	params, err := r.insertParams(ctx, t)
	if err != nil {
		return domain.Transaction{}, err
	}
	row, err := r.repos.queries.InsertTransaction(ctx, params)
	if err != nil {
		return domain.Transaction{}, mapConstraint(err)
	}
	return toTransaction(row)
}

func (r transactionRepo) Update(ctx context.Context, t domain.Transaction) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	insert, err := r.insertParams(ctx, t)
	if err != nil {
		return err
	}
	params := toUpdateTransactionParams(t, insert.SearchText)
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

func (r transactionRepo) List(ctx context.Context, f domain.TxFilter, after *app.TxCursor, limit int) ([]domain.Transaction, error) {
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

// insertParams builds the write parameters, looking up the payee whose name is part of the search text.
func (r transactionRepo) insertParams(ctx context.Context, t domain.Transaction) (gen.InsertTransactionParams, error) {
	var payee string
	if t.PayeeID != 0 {
		row, err := r.repos.queries.GetPayee(ctx, int64(t.PayeeID))
		if err != nil {
			return gen.InsertTransactionParams{}, mapNotFound(mapSQLError(err), "transaction payee")
		}
		payee = row.Name
	}
	return toInsertTransactionParams(t, payee), nil
}

func toInsertTransactionParams(t domain.Transaction, payeeName string) gen.InsertTransactionParams {
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
		SearchText:      textnorm.SearchText(t.Description, payeeName, t.Notes),
	}
}

func toUpdateTransactionParams(t domain.Transaction, searchText string) gen.UpdateTransactionParams {
	params := toInsertTransactionParams(t, "")
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
		SearchText:      searchText,
	}
}

// ftsMinRunes is the shortest text the trigram index can match; shorter texts use instr.
const ftsMinRunes = 3

func listTransactionParams(f domain.TxFilter, after *app.TxCursor, limit int) (gen.ListTransactionsParams, error) {
	accountIDsJSON, err := encodeIDSlice(f.AccountIDs())
	if err != nil {
		return gen.ListTransactionsParams{}, err
	}
	categoryIDsJSON, err := encodeIDSlice(f.CategoryIDs())
	if err != nil {
		return gen.ListTransactionsParams{}, err
	}
	payeeIDsJSON, err := encodeIDSlice(f.PayeeIDs())
	if err != nil {
		return gen.ListTransactionsParams{}, err
	}
	statusesJSON, err := encodeJSON(stringsOf(f.Statuses()))
	if err != nil {
		return gen.ListTransactionsParams{}, err
	}
	kindsJSON, err := encodeJSON(stringsOf(f.Kinds()))
	if err != nil {
		return gen.ListTransactionsParams{}, err
	}

	if f.Limit() > 0 && (limit <= 0 || f.Limit() < limit) {
		limit = f.Limit()
	}
	params := gen.ListTransactionsParams{
		FromDate:        dateStringOrEmpty(f.From()),
		ToDate:          dateStringOrEmpty(f.To()),
		AccountIdsJson:  accountIDsJSON,
		CategoryIdsJson: categoryIDsJSON,
		PayeeIdsJson:    payeeIDsJSON,
		StatusesJson:    statusesJSON,
		KindsJson:       kindsJSON,
		ImportBatchID:   int64(f.ImportBatchID()),
		Sign:            string(f.Sign()),
		SortKey:         string(f.Sort()),
		LimitValue:      int64(limit),
		// sqlc types these flags as interface{}; nil would make the SQL comparisons NULL.
		Uncategorized: int64(0), MinAbsSet: int64(0), MaxAbsSet: int64(0), MaxAbs: int64(0),
		TextMode: int64(0), AfterSet: int64(0), Desc: int64(0),
	}
	if f.Uncategorized() {
		params.Uncategorized = int64(1)
	}
	if f.Desc() {
		params.Desc = int64(1)
	}
	if v := f.MinAbs(); v != nil {
		params.MinAbsSet, params.MinAbs = int64(1), *v
	}
	if v := f.MaxAbs(); v != nil {
		params.MaxAbsSet, params.MaxAbs = int64(1), *v
	}
	setTextParams(&params, f.Text())
	// The keyset cursor only makes sense for the date sort; amount and payee page by limit only.
	if after != nil && f.Sort() == domain.TxSortDate {
		params.AfterSet = int64(1)
		params.AfterDate = after.Date.String()
		params.AfterID = int64(after.ID)
	}
	return params, nil
}

// setTextParams normalizes the text exactly like the indexed column (textnorm) and picks the
// trigram index when it can match, instr otherwise.
func setTextParams(params *gen.ListTransactionsParams, text string) {
	text = textnorm.Normalize(text)
	switch {
	case text == "":
	case utf8.RuneCountInString(text) < ftsMinRunes:
		params.TextMode, params.Text = int64(1), text
	default:
		// A quoted FTS5 string is a phrase; for the trigram tokenizer that is a substring match.
		params.TextMode, params.FtsQuery = int64(2), `"`+strings.ReplaceAll(text, `"`, `""`)+`"`
	}
}

func stringsOf[T ~string](in []T) []string {
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = string(v)
	}
	return out
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
