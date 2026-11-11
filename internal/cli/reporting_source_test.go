package cli_test

import (
	"context"
	"fmt"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/testutil/memstore"
)

// reportingSource adapts memstore.Store to reporting.Source for tests, pending the real store
// adapter (owned by the SQLite store task).
type reportingSource struct {
	store *memstore.Store
}

func (s reportingSource) ListAccounts(ctx context.Context, filter reporting.AccountFilter) ([]domain.Account, error) {
	var out []domain.Account
	err := s.store.Read(ctx, func(r app.Repos) error {
		all, err := r.Accounts().List(ctx)
		if err != nil {
			return err
		}
		ids := toIDSet(filter.AccountIDs)
		for _, a := range all {
			if !filter.IncludeArchived && a.Archived {
				continue
			}
			if len(ids) > 0 && !ids[a.ID] {
				continue
			}
			out = append(out, a)
		}
		return nil
	})
	return out, err
}

func (s reportingSource) ListCategories(ctx context.Context, categoryIDs []domain.CategoryID) ([]domain.Category, error) {
	if len(categoryIDs) == 0 {
		return nil, nil
	}
	wanted := make(map[domain.CategoryID]bool, len(categoryIDs))
	for _, id := range categoryIDs {
		wanted[id] = true
	}
	var out []domain.Category
	err := s.store.Read(ctx, func(r app.Repos) error {
		all, err := r.Categories().List(ctx)
		if err != nil {
			return err
		}
		for _, c := range all {
			if wanted[c.ID] {
				out = append(out, c)
			}
		}
		return nil
	})
	return out, err
}

func (s reportingSource) ListTransactions(ctx context.Context, filter reporting.TransactionFilter) ([]domain.Transaction, error) {
	var out []domain.Transaction
	err := s.store.Read(ctx, func(r app.Repos) error {
		txs, err := r.Transactions().List(ctx, app.TxFilter{
			From: filter.Period.From(), To: filter.Period.To(), AccountIDs: filter.AccountIDs,
		}, nil, 0)
		out = txs
		return err
	})
	return out, err
}

func (s reportingSource) BalanceAt(ctx context.Context, accountID domain.AccountID, at domain.Date) (domain.Money, error) {
	var out domain.Money
	err := s.store.Read(ctx, func(r app.Repos) error {
		a, err := r.Accounts().Get(ctx, accountID)
		if err != nil {
			return err
		}
		sum, err := r.Transactions().SumUpTo(ctx, accountID, at)
		if err != nil {
			return err
		}
		moves, err := domain.NewMoney(sum, a.Currency)
		if err != nil {
			return err
		}
		out, err = a.OpeningBalance.Add(moves)
		return err
	})
	return out, err
}

func (s reportingSource) FXRateOnOrBefore(ctx context.Context, from, to domain.Currency, at domain.Date) (domain.Rate, error) {
	var out domain.Rate
	err := s.store.Read(ctx, func(r app.Repos) error {
		fx, err := r.FXRates().OnOrBefore(ctx, from, at)
		if err != nil {
			return fmt.Errorf("fx rate %s->%s on %s: %w", from, to, at, err)
		}
		out = fx.Rate
		return nil
	})
	return out, err
}

func toIDSet(ids []domain.AccountID) map[domain.AccountID]bool {
	set := make(map[domain.AccountID]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}
