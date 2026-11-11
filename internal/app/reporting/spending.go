package reporting

import (
	"context"
	"fmt"

	"github.com/0xballo/fui/internal/domain"
)

// UncategorizedCategoryName is the stable label used for uncategorized spending.
const UncategorizedCategoryName = "Uncategorized"

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
	CategoryName  string
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
	categoryNames, err := useCase.loadCategoryNames(ctx, rows)
	if err != nil {
		return SpendingResult{}, err
	}
	return SpendingResult{
		Period:   period,
		Currency: params.BaseCurrency,
		Rows:     mapSpendingRows(rows, categoryNames),
		Total:    total,
	}, nil
}

func (useCase Spending) loadCategoryNames(ctx context.Context, rows []domain.CategoryTotal) (map[domain.CategoryID]string, error) {
	ids := make([]domain.CategoryID, 0, len(rows))
	for _, row := range rows {
		if row.Uncategorized {
			continue
		}
		ids = append(ids, row.CategoryID)
	}
	categories, err := useCase.deps.source.ListCategories(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	names := make(map[domain.CategoryID]string, len(categories))
	for _, category := range categories {
		names[category.ID] = category.Name
	}
	for _, id := range ids {
		if _, ok := names[id]; !ok {
			return nil, fmt.Errorf("missing category %d for spending report: %w", id, domain.ErrNotFound)
		}
	}
	return names, nil
}

func mapSpendingRows(rows []domain.CategoryTotal, categoryNames map[domain.CategoryID]string) []SpendingRow {
	out := make([]SpendingRow, 0, len(rows))
	for _, row := range rows {
		name := UncategorizedCategoryName
		if !row.Uncategorized {
			name = categoryNames[row.CategoryID]
		}
		out = append(out, SpendingRow{
			CategoryID:    row.CategoryID,
			CategoryName:  name,
			Uncategorized: row.Uncategorized,
			Total:         row.Total,
		})
	}
	return out
}
