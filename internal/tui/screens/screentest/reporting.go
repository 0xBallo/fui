package screentest

import (
	"context"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/domain"
)

// reportingSource adapts a Fixture's store into reporting.Source, the same translation a real
// composition root performs between the store and the reporting package, so dashboard tests
// exercise the actual Spending/Runway computations instead of a stub.
type reportingSource struct {
	store app.Store
}

func (s reportingSource) ListAccounts(ctx context.Context, filter reporting.AccountFilter) ([]domain.Account, error) {
	var out []domain.Account
	err := s.store.Read(ctx, func(r app.Repos) error {
		all, err := r.Accounts().List(ctx)
		if err != nil {
			return err
		}
		for _, a := range all {
			if !filter.IncludeArchived && a.Archived {
				continue
			}
			if len(filter.AccountIDs) > 0 && !containsID(filter.AccountIDs, a.ID) {
				continue
			}
			out = append(out, a)
		}
		return nil
	})
	return out, err
}

func (s reportingSource) ListCategories(ctx context.Context, categoryIDs []domain.CategoryID) ([]domain.Category, error) {
	var out []domain.Category
	err := s.store.Read(ctx, func(r app.Repos) error {
		all, err := r.Categories().List(ctx)
		if err != nil {
			return err
		}
		if len(categoryIDs) == 0 {
			out = all
			return nil
		}
		for _, c := range all {
			if containsID(categoryIDs, c.ID) {
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
		f, err := domain.NewTxFilter(domain.TxFilterSpec{
			From: filter.Period.From(), To: filter.Period.To(), AccountIDs: filter.AccountIDs, Desc: true,
		})
		if err != nil {
			return err
		}
		txs, err := r.Transactions().List(ctx, f, nil, 0)
		out = txs
		return err
	})
	return out, err
}

func (s reportingSource) BalanceAt(ctx context.Context, accountID domain.AccountID, date domain.Date) (domain.Money, error) {
	var out domain.Money
	err := s.store.Read(ctx, func(r app.Repos) error {
		a, err := r.Accounts().Get(ctx, accountID)
		if err != nil {
			return err
		}
		sum, err := r.Transactions().SumUpTo(ctx, a.ID, date)
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

func (s reportingSource) FXRateOnOrBefore(ctx context.Context, from, _ domain.Currency, date domain.Date) (domain.Rate, error) {
	var out domain.Rate
	err := s.store.Read(ctx, func(r app.Repos) error {
		fx, err := r.FXRates().OnOrBefore(ctx, from, date)
		out = fx.Rate
		return err
	})
	return out, err
}

func containsID[T comparable](ids []T, id T) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

// reportingClock adapts fixedClock (already Now()-shaped) to reporting.Clock; the two interfaces
// are structurally identical, so this alias documents the intent rather than converting data.
type reportingClock = fixedClock

// Reporting bundles the Spending and Runway use cases wired over the fixture's store, for screens
// (such as the dashboard) that need the app/reporting package in addition to app.Service.
type Reporting struct {
	spending reporting.Spending
	runway   reporting.Runway
}

// NewReporting builds the Spending and Runway use cases over f's store, frozen at the same instant
// as f.Service.
func (f Fixture) NewReporting(t testingT) Reporting {
	t.Helper()
	src := reportingSource{store: f.Store}
	clock := reportingClock{now: f.now}
	spending, err := reporting.NewSpending(src, clock)
	if err != nil {
		t.Fatalf("NewSpending: %v", err)
	}
	runway, err := reporting.NewRunway(src, clock)
	if err != nil {
		t.Fatalf("NewRunway: %v", err)
	}
	return Reporting{spending: spending, runway: runway}
}

// Spending runs the spending-by-category report.
func (r Reporting) Spending(ctx context.Context, params reporting.SpendingParams) (reporting.SpendingResult, error) {
	return r.spending.Run(ctx, params)
}

// Runway runs the runway KPI.
func (r Reporting) Runway(ctx context.Context, params reporting.RunwayParams) (reporting.RunwayResult, error) {
	return r.runway.Run(ctx, params)
}

// testingT is the subset of *testing.T this package needs, so helpers that only have a *testing.T
// (not the full Fixture-building New) can still report fatal failures.
type testingT interface {
	Helper()
	Fatalf(format string, args ...any)
}
