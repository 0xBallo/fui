package memstore

import (
	"context"
	"slices"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/textnorm"
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
	if err := t.checkPayee(tx); err != nil {
		return domain.Transaction{}, err
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
	if err := t.checkPayee(tx); err != nil {
		return err
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
	delete(t.st.TxTags, id) // ON DELETE CASCADE in the SQLite store
	delete(t.st.Splits, id)
	for key, linked := range t.st.InstallmentLinks {
		if linked == id {
			delete(t.st.InstallmentLinks, key)
		}
	}
	return nil
}

func (t transactions) Get(_ context.Context, id domain.TransactionID) (domain.Transaction, error) {
	tx, ok := t.st.Txs[id]
	if !ok {
		return domain.Transaction{}, notFound("transaction", id)
	}
	return tx, nil
}

func (t transactions) checkPayee(tx domain.Transaction) error {
	if tx.PayeeID == 0 {
		return nil
	}
	if _, ok := t.st.Payees[tx.PayeeID]; !ok {
		return notFound("payee", tx.PayeeID)
	}
	return nil
}

// searchText mirrors what the SQLite store indexes: description, payee name and notes.
func (t transactions) searchText(tx domain.Transaction) string {
	return textnorm.SearchText(tx.Description, t.st.Payees[tx.PayeeID].Name, tx.Notes)
}

// List is the oracle of the SQL translation: filter with TxFilter.Match, order with TxFilter.Less.
func (t transactions) List(_ context.Context, f domain.TxFilter, after *app.TxCursor, limit int) ([]domain.Transaction, error) {
	var out []domain.Transaction
	for _, tx := range t.st.Txs {
		if f.Match(tx, t.searchText(tx)) {
			out = append(out, tx)
		}
	}
	slices.SortFunc(out, func(a, b domain.Transaction) int {
		switch {
		case f.Less(a, b, "", ""):
			return -1
		case f.Less(b, a, "", ""):
			return 1
		}
		return 0
	})
	if after != nil && f.Sort() == domain.TxSortDate {
		cur := domain.Transaction{Date: after.Date, ID: after.ID}
		out = slices.DeleteFunc(out, func(tx domain.Transaction) bool { return !f.Less(cur, tx, "", "") })
	}
	if f.Limit() > 0 && (limit <= 0 || f.Limit() < limit) {
		limit = f.Limit()
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
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
