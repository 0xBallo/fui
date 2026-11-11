package memstore

import (
	"context"
	"slices"
	"strings"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

type transactions struct{ *repos }

func (t transactions) checkExternalID(tx domain.Transaction) error {
	if tx.ExternalID == "" {
		return nil
	}
	for _, o := range t.st.Txs {
		if o.ID != tx.ID && o.AccountID == tx.AccountID && o.ExternalID == tx.ExternalID {
			return exists("external id", tx.ExternalID)
		}
	}
	return nil
}

func (t transactions) Insert(_ context.Context, tx domain.Transaction) (domain.Transaction, error) {
	if err := t.writable(); err != nil {
		return domain.Transaction{}, err
	}
	if _, ok := t.st.Accounts[tx.AccountID]; !ok {
		return domain.Transaction{}, notFound("account", tx.AccountID)
	}
	tx.ID = 0
	if err := t.checkExternalID(tx); err != nil {
		return domain.Transaction{}, err
	}
	tx.ID = domain.TransactionID(t.st.nextID("transactions"))
	t.st.Txs[tx.ID] = tx
	return tx, nil
}

func (t transactions) Update(_ context.Context, tx domain.Transaction) error {
	if err := t.writable(); err != nil {
		return err
	}
	if _, ok := t.st.Txs[tx.ID]; !ok {
		return notFound("transaction", tx.ID)
	}
	if err := t.checkExternalID(tx); err != nil {
		return err
	}
	t.st.Txs[tx.ID] = tx
	return nil
}

func (t transactions) Delete(_ context.Context, id domain.TransactionID) error {
	if err := t.writable(); err != nil {
		return err
	}
	if _, ok := t.st.Txs[id]; !ok {
		return notFound("transaction", id)
	}
	delete(t.st.Txs, id)
	return nil
}

func (t transactions) Get(_ context.Context, id domain.TransactionID) (domain.Transaction, error) {
	tx, ok := t.st.Txs[id]
	if !ok {
		return domain.Transaction{}, notFound("transaction", id)
	}
	return tx, nil
}

// newestFirst orders by (date, id) descending.
func newestFirst(a, b domain.Transaction) int {
	if c := b.Date.Cmp(a.Date); c != 0 {
		return c
	}
	switch {
	case a.ID < b.ID:
		return 1
	case a.ID > b.ID:
		return -1
	}
	return 0
}

func (t transactions) List(_ context.Context, f app.TxFilter, after *app.TxCursor, limit int) ([]domain.Transaction, error) {
	var out []domain.Transaction
	for _, tx := range t.st.Txs {
		if matches(tx, f) {
			out = append(out, tx)
		}
	}
	slices.SortFunc(out, newestFirst)
	if after != nil {
		cur := domain.Transaction{Date: after.Date, ID: after.ID}
		out = slices.DeleteFunc(out, func(tx domain.Transaction) bool { return newestFirst(tx, cur) <= 0 })
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func matches(tx domain.Transaction, f app.TxFilter) bool {
	switch {
	case !f.From.IsZero() && tx.Date.Before(f.From),
		!f.To.IsZero() && tx.Date.After(f.To),
		len(f.AccountIDs) > 0 && !slices.Contains(f.AccountIDs, tx.AccountID),
		len(f.CategoryIDs) > 0 && !slices.Contains(f.CategoryIDs, tx.CategoryID),
		len(f.Statuses) > 0 && !slices.Contains(f.Statuses, tx.Status),
		f.MinMinor != nil && tx.Amount.Minor() < *f.MinMinor,
		f.MaxMinor != nil && tx.Amount.Minor() > *f.MaxMinor,
		f.ImportBatchID != 0 && tx.ImportBatchID != f.ImportBatchID:
		return false
	}
	if f.Text == "" {
		return true
	}
	return strings.Contains(strings.ToLower(tx.Description+"\n"+tx.Notes), strings.ToLower(f.Text))
}

func (t transactions) SumUpTo(_ context.Context, id domain.AccountID, date domain.Date) (int64, error) {
	var sum int64
	for _, tx := range t.st.Txs {
		if tx.AccountID == id && !tx.Date.After(date) {
			sum += tx.Amount.Minor()
		}
	}
	return sum, nil
}

func (t transactions) where(keep func(domain.Transaction) bool) []domain.Transaction {
	var out []domain.Transaction
	for _, tx := range sortedValues(t.st.Txs) {
		if keep(tx) {
			out = append(out, tx)
		}
	}
	return out
}

func (t transactions) ByExternalID(_ context.Context, id domain.AccountID, ext string) ([]domain.Transaction, error) {
	return t.where(func(tx domain.Transaction) bool { return tx.AccountID == id && tx.ExternalID == ext }), nil
}

func (t transactions) ByFingerprint(_ context.Context, fp string) ([]domain.Transaction, error) {
	return t.where(func(tx domain.Transaction) bool { return tx.Fingerprint == fp }), nil
}

func (t transactions) NearAmount(_ context.Context, id domain.AccountID, minor int64, from, to domain.Date) ([]domain.Transaction, error) {
	return t.where(func(tx domain.Transaction) bool {
		return tx.AccountID == id && tx.Amount.Minor() == minor && !tx.Date.Before(from) && !tx.Date.After(to)
	}), nil
}

func (t transactions) ByTransferGroup(_ context.Context, id domain.TransferGroupID) ([]domain.Transaction, error) {
	return t.where(func(tx domain.Transaction) bool { return tx.TransferGroupID == id }), nil
}
