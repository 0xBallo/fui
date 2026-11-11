package reporting

import (
	"context"
	"fmt"
	"time"

	"github.com/0xballo/fui/internal/domain"
)

// Source is the reporting data source implemented by adapters (for example SQLite).
type Source interface {
	// ListAccounts returns accounts, optionally filtered by IDs.
	ListAccounts(ctx context.Context, filter AccountFilter) ([]domain.Account, error)
	// ListCategories returns categories for the given IDs.
	ListCategories(ctx context.Context, categoryIDs []domain.CategoryID) ([]domain.Category, error)
	// ListTransactions returns transactions in period, optionally filtered by accounts.
	ListTransactions(ctx context.Context, filter TransactionFilter) ([]domain.Transaction, error)
	// BalanceAt returns the account balance at date.
	BalanceAt(ctx context.Context, accountID domain.AccountID, date domain.Date) (domain.Money, error)
	// FXRateOnOrBefore returns the FX rate valid on or before date.
	FXRateOnOrBefore(ctx context.Context, from domain.Currency, to domain.Currency, date domain.Date) (domain.Rate, error)
}

// Clock supplies current wall-clock time.
type Clock interface {
	// Now returns the current instant.
	Now() time.Time
}

// AccountFilter selects accounts for reporting.
type AccountFilter struct {
	AccountIDs      []domain.AccountID
	IncludeArchived bool
}

// TransactionFilter selects transactions for reporting.
type TransactionFilter struct {
	Period     domain.Period
	AccountIDs []domain.AccountID
}

type dependencies struct {
	source Source
	clock  Clock
}

func validateDependencies(source Source, clock Clock) (dependencies, error) {
	if source == nil {
		return dependencies{}, fmt.Errorf("missing source: %w", domain.ErrMissingField)
	}
	if clock == nil {
		return dependencies{}, fmt.Errorf("missing clock: %w", domain.ErrMissingField)
	}
	return dependencies{source: source, clock: clock}, nil
}

func resolvePeriod(period domain.Period, now time.Time, monthStartDay int) (domain.Period, error) {
	if monthStartDay == 0 {
		monthStartDay = 1
	}
	if !period.From().IsZero() && !period.To().IsZero() {
		return period, nil
	}
	today, err := domain.FromTime(now)
	if err != nil {
		return domain.Period{}, err
	}
	return domain.MonthPeriod(today, monthStartDay)
}
