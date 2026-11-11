package reporting

import (
	"context"
	"fmt"

	"github.com/0xballo/fui/internal/domain"
)

// Spending computes the spending-by-category report.
type Spending struct {
	deps dependencies
}

// NewSpending creates a Spending use case.
func NewSpending(source Source, clock Clock) (Spending, error) {
	deps, err := validateDependencies(source, clock)
	if err != nil {
		return Spending{}, err
	}
	return Spending{deps: deps}, nil
}

// SpendingParams are the query parameters.
type SpendingParams struct {
	Period        domain.Period
	MonthStartDay int
	AccountIDs    []domain.AccountID
	BaseCurrency  domain.Currency
}

// SpendingRow is one category line.
type SpendingRow struct {
	CategoryID    domain.CategoryID
	Uncategorized bool
	Total         domain.Money
}

// SpendingResult is the structured spending report.
type SpendingResult struct {
	Period   domain.Period
	Currency domain.Currency
	Rows     []SpendingRow
	Total    domain.Money
}

// Run executes the spending-by-category report.
func (useCase Spending) Run(ctx context.Context, params SpendingParams) (SpendingResult, error) {
	period, err := resolvePeriod(params.Period, useCase.deps.clock.Now(), params.MonthStartDay)
	if err != nil {
		return SpendingResult{}, fmt.Errorf("resolve period: %w", err)
	}
	filter := TransactionFilter{Period: period, AccountIDs: params.AccountIDs}
	transactions, err := useCase.deps.source.ListTransactions(ctx, filter)
	if err != nil {
		return SpendingResult{}, fmt.Errorf("list transactions: %w", err)
	}
	rows, total, err := domain.SpendingByCategory(period, transactions, params.BaseCurrency, useCase.deps.fxLookup(ctx))
	if err != nil {
		return SpendingResult{}, fmt.Errorf("compute spending report: %w", err)
	}
	return SpendingResult{
		Period:   period,
		Currency: params.BaseCurrency,
		Rows:     mapSpendingRows(rows),
		Total:    total,
	}, nil
}

func mapSpendingRows(rows []domain.CategoryTotal) []SpendingRow {
	out := make([]SpendingRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, SpendingRow{
			CategoryID:    row.CategoryID,
			Uncategorized: row.Uncategorized,
			Total:         row.Total,
		})
	}
	return out
}
