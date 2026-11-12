package memstore

import (
	"context"
	"fmt"
	"slices"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

type splits struct{ *repos }

func (s splits) SplitsOf(_ context.Context, tx domain.TransactionID) ([]domain.Split, error) {
	if _, ok := s.st.Txs[tx]; !ok {
		return nil, notFound("transaction", tx)
	}
	return slices.Clone(s.st.Splits[tx]), nil
}

func (s splits) ReplaceSplits(_ context.Context, tx domain.TransactionID, rows []domain.Split) error {
	if err := s.writable(); err != nil {
		return err
	}
	t, ok := s.st.Txs[tx]
	if !ok {
		return notFound("transaction", tx)
	}
	if len(rows) > 0 {
		if err := domain.ValidateSplits(t.Amount, rows); err != nil {
			return fmt.Errorf("transaction %d: %w", tx, err)
		}
	}
	for _, r := range rows {
		if _, ok := s.st.Categories[r.Category]; !ok {
			return notFound("split category", r.Category)
		}
	}
	if len(rows) == 0 {
		delete(s.st.Splits, tx)
		return nil
	}
	s.st.Splits[tx] = slices.Clone(rows)
	return nil
}

var _ app.SplitRepo = splits{}
