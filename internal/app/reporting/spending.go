package reporting

import (
	"context"
	"fmt"

	"github.com/0xballo/fui/internal/app"
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

// Spending reports what was spent in the period, in the base currency, grouped by req.GroupBy
// (category by default) over the rows matching req.Filter. Only outflows count (refunds are not
// netted); transfers and investments are left out; split transactions count per split row.
// Rows are by descending total, period groupings chronologically. Grouping by tag counts a
// transaction fully in each of its tags and in an "untagged" row, so tag rows may sum above
// the total; the other groupings sum to it.
func (a *Analytics) Spending(ctx context.Context, req ReportRequest) (GroupedResult, error) {
	if err := a.gate(); err != nil {
		return GroupedResult{}, err
	}
	by, err := reportGroup(req.GroupBy, domain.GroupCategory)
	if err != nil {
		return GroupedResult{}, err
	}
	p, err := a.period(req.Period)
	if err != nil {
		return GroupedResult{}, err
	}
	out := GroupedResult{Period: periodLabel(p), GroupBy: string(by), Rows: []GroupedRow{}}
	err = a.store.Read(ctx, func(r app.Repos) error {
		rows, err := a.reportRows(ctx, r, req, p, rowOptions{excludeKinds: transferKinds, withTags: by == domain.GroupTag})
		if err != nil {
			return err
		}
		g, err := a.newGrouper(ctx, r, by, p)
		if err != nil {
			return err
		}
		total := zeroMoney(a.cfg.BaseCurrency)
		for _, row := range rows {
			base, err := a.toBase(ctx, r, row.Amount, row.Date)
			if err != nil {
				return err
			}
			if base.Sign() >= 0 {
				continue
			}
			spent, err := base.Neg()
			if err != nil {
				return err
			}
			if total, err = total.Add(spent); err != nil {
				return err
			}
			if err := g.add(row, base); err != nil {
				return err
			}
		}
		if out.Rows, err = g.result(true); err != nil {
			return err
		}
		out.Total = amountOf(total)
		return nil
	})
	return out, err
}
