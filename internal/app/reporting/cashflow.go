package reporting

import (
	"context"
	"fmt"

	"github.com/0xballo/fui/internal/domain"
)

// CashFlow computes the monthly cash-flow report.
type CashFlow struct {
	deps dependencies
}

// NewCashFlow creates a CashFlow use case.
func NewCashFlow(source Source, clock Clock) (CashFlow, error) {
	deps, err := validateDependencies(source, clock)
	if err != nil {
		return CashFlow{}, err
	}
	return CashFlow{deps: deps}, nil
}

// CashFlowParams are the query parameters.
type CashFlowParams struct {
	Period        domain.Period
	MonthStartDay int
	AccountIDs    []domain.AccountID
	BaseCurrency  domain.Currency
}

// CashFlowMonth is one accounting month in the result.
type CashFlowMonth struct {
	Period  domain.Period
	Inflow  domain.Money
	Outflow domain.Money
	Net     domain.Money
}

// CashFlowResult is the structured cash-flow report.
type CashFlowResult struct {
	Period       domain.Period
	Currency     domain.Currency
	Months       []CashFlowMonth
	TotalInflow  domain.Money
	TotalOutflow domain.Money
	TotalNet     domain.Money
}

// Run executes the cash-flow report.
func (useCase CashFlow) Run(ctx context.Context, params CashFlowParams) (CashFlowResult, error) {
	period, err := resolvePeriod(params.Period, useCase.deps.clock.Now(), params.MonthStartDay)
	if err != nil {
		return CashFlowResult{}, fmt.Errorf("resolve period: %w", err)
	}
	filter := TransactionFilter{Period: period, AccountIDs: params.AccountIDs}
	transactions, err := useCase.deps.source.ListTransactions(ctx, filter)
	if err != nil {
		return CashFlowResult{}, fmt.Errorf("list transactions: %w", err)
	}
	rows, err := domain.CashFlowByMonth(period, params.MonthStartDay, transactions, params.BaseCurrency, useCase.deps.fxLookup(ctx))
	if err != nil {
		return CashFlowResult{}, fmt.Errorf("compute cash-flow report: %w", err)
	}
	result := CashFlowResult{
		Period:   period,
		Currency: params.BaseCurrency,
		Months:   mapCashFlowMonths(rows),
	}
	if err := result.computeTotals(); err != nil {
		return CashFlowResult{}, err
	}
	return result, nil
}

func mapCashFlowMonths(rows []domain.CashFlowMonth) []CashFlowMonth {
	out := make([]CashFlowMonth, 0, len(rows))
	for _, row := range rows {
		out = append(out, CashFlowMonth{
			Period:  row.Period,
			Inflow:  row.Inflow,
			Outflow: row.Outflow,
			Net:     row.Net,
		})
	}
	return out
}

func (result *CashFlowResult) computeTotals() error {
	totalInflow, err := domain.NewMoney(0, result.Currency)
	if err != nil {
		return fmt.Errorf("init inflow total: %w", err)
	}
	totalOutflow, err := domain.NewMoney(0, result.Currency)
	if err != nil {
		return fmt.Errorf("init outflow total: %w", err)
	}
	totalNet, err := domain.NewMoney(0, result.Currency)
	if err != nil {
		return fmt.Errorf("init net total: %w", err)
	}
	for _, month := range result.Months {
		totalInflow, err = totalInflow.Add(month.Inflow)
		if err != nil {
			return fmt.Errorf("sum inflow: %w", err)
		}
		totalOutflow, err = totalOutflow.Add(month.Outflow)
		if err != nil {
			return fmt.Errorf("sum outflow: %w", err)
		}
		totalNet, err = totalNet.Add(month.Net)
		if err != nil {
			return fmt.Errorf("sum net: %w", err)
		}
	}
	result.TotalInflow = totalInflow
	result.TotalOutflow = totalOutflow
	result.TotalNet = totalNet
	return nil
}
